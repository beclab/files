package handlers

import (
	"bytes"
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/sync/seahub"
	"files/pkg/models"
	"files/pkg/utils"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

// todo check file same name before
func (c *Handler) UploadToCloud() error {
	klog.Infof("UploadToCloud - owner: %s, action: %s, src: %s, dst: %s", c.owner, c.action, utils.ToJson(c.src), utils.ToJson(c.dst))

	return c.cloudTransfer()

}

func (c *Handler) UploadToSync() error {
	klog.Infof("~~~Copy Debug log: Upload to sync begins!")
	header := make(http.Header)
	header.Add("X-Bfl-User", c.owner)

	totalSize, err := c.GetToSyncFileCount(header, "size") // file and dir can both use this
	if err != nil {
		klog.Errorf("UploadToSync - GetFromSyncFileCount - %v", err)
		return err
	}
	klog.Infof("~~~Copy Debug log: UploadToSync - GetFromSyncFileCount - totalSize: %d", totalSize)
	if totalSize == 0 {
		return errors.New("DownloadFromSync - GetFromSyncFileCount - empty total size")
	}
	c.UpdateTotalSize(totalSize)

	_, isFile := c.src.IsFile()
	if isFile {
		err = c.UploadFileToSync(header, nil, nil)
		if err != nil {
			return err
		}
	} else {
		err = c.UploadDirToSync(header, nil, nil)
		if err != nil {
			return err
		}
	}
	//if c.action == "move" {
	//	err = (header, c.src)
	//	if err != nil {
	//		return err
	//	}
	//}
	_, _, transferred, _ := c.GetProgress()
	c.UpdateProgress(100, transferred)
	return nil
}

func (c *Handler) GetToSyncFileCount(header http.Header, countType string) (int64, error) {
	uri, err := c.src.GetResourceUri()
	if err != nil {
		return 0, err
	}
	newSrc := uri + c.src.Path

	srcinfo, err := os.Stat(newSrc)
	if err != nil {
		return 0, err
	}

	var count int64 = 0

	if srcinfo.IsDir() {
		err = filepath.Walk(newSrc, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() {
				if countType == "size" {
					count += info.Size()
				} else {
					count++
				}
			}
			return nil
		})

		if err != nil {
			klog.Infoln("Error walking the directory:", err)
			return 0, err
		}
		klog.Infoln("Directory traversal completed.")
	} else {
		if countType == "size" {
			count = srcinfo.Size()
		} else {
			count = 1
		}
	}
	return count, nil
}

func (c *Handler) UploadDirToSync(header http.Header, src, dst *models.FileParam) error {
	select {
	case <-c.ctx.Done():
		return nil
	default:
	}

	if src == nil {
		src = c.src
	}
	if dst == nil {
		dst = c.dst
	}

	srcUri, err := src.GetResourceUri()
	if err != nil {
		return err
	}
	srcFullPath := srcUri + src.Path

	dstUri, err := dst.GetResourceUri()
	if err != nil {
		return err
	}
	dstFullPath := dstUri + dst.Path

	res, err := seahub.HandleDirOperation(header, dst.Extend, dst.Path, "", "mkdir")
	if err != nil {
		klog.Errorf("Sync create error: %v, path: %s", err, dst.Path)
		return err
	}
	klog.Infof("Sync create success, result: %s, path: %s", string(res), dst.Path)

	var fdstBase string = dstFullPath

	dir, _ := os.Open(srcFullPath)
	obs, err := dir.Readdir(-1)
	if err != nil {
		return err
	}

	var errs []error

	for _, obj := range obs {
		select {
		case <-c.ctx.Done():
			return nil
		default:
		}

		fsrc := filepath.Join(srcFullPath, obj.Name())
		fdst := filepath.Join(fdstBase, obj.Name())

		fsrcFileParam := &models.FileParam{
			Owner:    src.Owner,
			FileType: src.FileType,
			Extend:   src.Extend,
			Path:     fsrc,
		}
		fdstFileParam := &models.FileParam{
			Owner:    dst.Owner,
			FileType: dst.FileType,
			Extend:   dst.Extend,
			Path:     fdst,
		}

		if obj.IsDir() {
			// Create sub-directories, recursively.
			err = c.UploadDirToSync(header, fsrcFileParam, fdstFileParam)
			if err != nil {
				errs = append(errs, err)
			}
		} else {
			// Perform the file copy.
			err = c.UploadFileToSync(header, fsrcFileParam, fdstFileParam)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	var errString string
	for _, err = range errs {
		errString += err.Error() + "\n"
	}

	if errString != "" {
		return errors.New(errString)
	}
	return nil
}

func (c *Handler) UploadFileToSync(header http.Header, src, dst *models.FileParam) error {
	klog.Infof("~~~Copy Debug log: Download file from sync begins!")
	select {
	case <-c.ctx.Done():
		return nil
	default:
	}

	if src == nil {
		src = c.src
	}
	if dst == nil {
		dst = c.dst
	}

	srcUri, err := src.GetResourceUri()
	if err != nil {
		return err
	}
	srcFullPath := srcUri + src.Path

	srcinfo, err := os.Stat(srcFullPath)
	if err != nil {
		return err
	}
	diskSize := srcinfo.Size()

	left, _, right := c.CalculateSyncProgressRange(diskSize)

	//repoId := dst.Extend
	prefix, filename := filepath.Split(dst.Path)
	prefix = strings.TrimPrefix(prefix, "/")

	extension := path.Ext(filename)
	mimeType := "application/octet-stream"
	if extension != "" {
		mimeType = mime.TypeByExtension(extension)
	}

	uploadParam := &models.FileParam{
		Owner:    dst.Owner,
		FileType: dst.FileType,
		Extend:   dst.Extend,
		Path:     filepath.Dir(dst.Path),
	}
	uploadLink, err := seahub.GetUploadLink(header, uploadParam, "api", false)
	if err != nil {
		return err
	}
	uploadLink = strings.Trim(uploadLink, "\"")

	targetURL := "http://127.0.0.1:80" + uploadLink + "?ret-json=1"
	klog.Infoln(targetURL)

	srcFile, err := os.Open(srcFullPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	chunkSize := int64(8 * 1024 * 1024) // 8MB
	totalChunks := (diskSize + chunkSize - 1) / chunkSize
	identifier := seahub.GenerateUniqueIdentifier(common.EscapeAndJoin(filename, "/"))

	var chunkStart int64 = 0
	for chunkNumber := int64(1); chunkNumber <= totalChunks; chunkNumber++ {
		select {
		case <-c.ctx.Done():
			return nil
		default:
		}

		status, _, transferred, _ := c.GetProgress()
		if status != "running" && status != "pending" {
			return nil
		}

		percent := (chunkNumber * 100) / totalChunks
		rangeSize := right - left
		mappedProgress := left + int((percent*int64(rangeSize))/100)
		finalProgress := mappedProgress
		if finalProgress < left {
			finalProgress = left
		} else if finalProgress > right {
			finalProgress = right
		}
		klog.Infof("finalProgress:%d", finalProgress)

		offset := (chunkNumber - 1) * chunkSize
		chunkData := make([]byte, chunkSize)
		bytesRead, err := srcFile.ReadAt(chunkData, offset)
		if err != nil && err != io.EOF {
			return err
		}

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		writer.WriteField("resumableChunkNumber", strconv.FormatInt(chunkNumber, 10))
		writer.WriteField("resumableChunkSize", strconv.FormatInt(chunkSize, 10))
		writer.WriteField("resumableCurrentChunkSize", strconv.FormatInt(int64(bytesRead), 10))
		writer.WriteField("resumableTotalSize", strconv.FormatInt(diskSize, 10))
		writer.WriteField("resumableType", mimeType)
		writer.WriteField("resumableIdentifier", identifier)
		writer.WriteField("resumableFilename", filename)
		writer.WriteField("resumableRelativePath", filename)
		writer.WriteField("resumableTotalChunks", strconv.FormatInt(totalChunks, 10))
		writer.WriteField("parent_dir", "/"+prefix)

		part, err := writer.CreateFormFile("file", common.EscapeAndJoin(filename, "/"))
		if err != nil {
			klog.Errorln("Create Form File error: ", err)
			return err
		}

		_, err = part.Write(chunkData[:bytesRead])
		if err != nil {
			klog.Errorln("Write Chunk Data error: ", err)
			return err
		}

		err = writer.Close()
		if err != nil {
			klog.Errorln("Write Close error: ", err)
			return err
		}

		request, err := http.NewRequest("POST", targetURL, body)
		if err != nil {
			klog.Errorln("New Request error: ", err)
			return err
		}

		request.Header = header.Clone()
		request.Header.Set("Content-Type", writer.FormDataContentType())
		request.Header.Set("Content-Disposition", "attachment; filename=\""+common.EscapeAndJoin(filename, "/")+"\"")
		request.Header.Set("Content-Range", "bytes "+strconv.FormatInt(chunkStart, 10)+"-"+strconv.FormatInt(chunkStart+int64(bytesRead)-1, 10)+"/"+strconv.FormatInt(diskSize, 10))
		chunkStart += int64(bytesRead)

		client := &http.Client{
			Timeout: 30 * time.Second,
		}

		maxRetries := 3
		var response *http.Response
		special := false

		for retry := 0; retry < maxRetries; retry++ {
			var req *http.Request
			var err error

			if retry == 0 {
				req, err = http.NewRequest(request.Method, request.URL.String(), request.Body)
				if err != nil {
					klog.Warningf("create request error: %v", err)
					continue
				}
				req.Header = make(http.Header)
				for k, s := range request.Header {
					req.Header[k] = s
				}
			} else {
				// newBody begin
				offset = (chunkNumber - 1) * chunkSize
				chunkData = make([]byte, chunkSize)
				bytesRead, err = srcFile.ReadAt(chunkData, offset)
				if err != nil && err != io.EOF {
					return err
				}

				newBody := &bytes.Buffer{}
				writer = multipart.NewWriter(newBody)

				writer.WriteField("resumableChunkNumber", strconv.FormatInt(chunkNumber, 10))
				writer.WriteField("resumableChunkSize", strconv.FormatInt(chunkSize, 10))
				writer.WriteField("resumableCurrentChunkSize", strconv.FormatInt(int64(bytesRead), 10))
				writer.WriteField("resumableTotalSize", strconv.FormatInt(diskSize, 10))
				writer.WriteField("resumableType", mimeType)
				writer.WriteField("resumableIdentifier", identifier)
				writer.WriteField("resumableFilename", filename)
				writer.WriteField("resumableRelativePath", filename)
				writer.WriteField("resumableTotalChunks", strconv.FormatInt(totalChunks, 10))
				writer.WriteField("parent_dir", "/"+prefix)

				part, err = writer.CreateFormFile("file", common.EscapeAndJoin(filename, "/"))
				if err != nil {
					klog.Errorln("Create Form File error: ", err)
					return err
				}

				_, err = part.Write(chunkData[:bytesRead])
				if err != nil {
					klog.Errorln("Write Chunk Data error: ", err)
					return err
				}

				err = writer.Close()
				if err != nil {
					klog.Errorln("Write Close error: ", err)
					return err
				}

				if err != nil {
					klog.Warningf("generate body error: %v", err)
					continue
				}
				// newBody end

				req, err = http.NewRequest(request.Method, request.URL.String(), newBody)
				if err != nil {
					klog.Warningf("create request error: %v", err)
					continue
				}
				req.Header = make(http.Header)
				for k, s := range request.Header {
					req.Header[k] = s
				}
			}

			response, err = client.Do(req)
			klog.Infoln("Do Request (attempt", retry+1, ")")

			if err != nil {
				klog.Warningf("request error (attempt %d): %v", retry+1, err)

				if chunkNumber == totalChunks {
					if strings.Contains(err.Error(), "context deadline exceeded (Client.Timeout exceeded while awaiting headers)") {
						const gb = 1024 * 1024 * 1024
						additionalBlocks := diskSize / (10 * gb)
						totalBubble := 15*time.Second + time.Duration(additionalBlocks)*15*time.Second
						klog.Infof("Waiting %ds for seafile to complete", int(totalBubble.Seconds()))
						time.Sleep(totalBubble)
						special = true
						if response != nil && response.Body != nil {
							response.Body.Close()
						}
						klog.Infof("Waiting for seafile to complete huge file done!")
						break
					}
				}

				if response != nil && response.Body != nil {
					bodyBytes, err := io.ReadAll(response.Body)
					if err != nil {
						klog.Warningf("read body error: %v", err)
					} else {
						bodyString := string(bodyBytes)
						klog.Infof("error response: %s", bodyString)

						response.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
					}
				} else {
					klog.Infof("got an empty error response")
				}

				if retry < maxRetries-1 {
					waitTime := time.Duration(1<<uint(retry)) * time.Second
					klog.Warningf("Retrying in %v...", waitTime)
					time.Sleep(waitTime)
				}
				continue
			}

			if response.StatusCode == http.StatusOK {
				break
			}

			klog.Warningf("non-200 status: %s (attempt %d)", response.Status, retry+1)

			if response.Body != nil {
				response.Body.Close()
			}

			if retry < maxRetries-1 {
				waitTime := time.Duration(1<<uint(retry)) * time.Second
				klog.Warningf("Retrying in %v...", waitTime)
				time.Sleep(waitTime)
			}
		}

		if !special {
			if response == nil || response.StatusCode != http.StatusOK {
				statusCode := http.StatusInternalServerError
				statusMsg := "request failed after retries"

				if response != nil {
					statusCode = response.StatusCode
					statusMsg = response.Status
					if response.Body != nil {
						defer response.Body.Close()
					}
				}

				klog.Warningf("%d, %s after %d attempts", statusCode, statusMsg, maxRetries)
				return fmt.Errorf("%d, %s after %d attempts", statusCode, statusMsg, maxRetries)
			}
			defer response.Body.Close()

			// Read the response body as a string
			postBody, err := io.ReadAll(response.Body)
			klog.Infoln("ReadAll")
			if err != nil {
				klog.Errorln("ReadAll error: ", err)
				return err
			}

			klog.Infoln("Status Code: ", response.StatusCode)
			if response.StatusCode != http.StatusOK {
				klog.Infoln(string(postBody))
				return fmt.Errorf("file upload failed, status code: %d", response.StatusCode)
			}
		}

		klog.Infof("Chunk %d/%d from of bytes %d-%d/%d successfully transferred.", chunkNumber, totalChunks, chunkStart, chunkStart+int64(bytesRead)-1, diskSize)
		c.UpdateProgress(finalProgress, transferred+chunkSize)

		time.Sleep(150 * time.Millisecond)
	}
	klog.Infoln("upload file to sync success!")

	_, _, transferred, _ := c.GetProgress()
	c.UpdateProgress(right, transferred)
	return nil
}
