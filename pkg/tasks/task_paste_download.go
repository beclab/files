package tasks

import (
	"bufio"
	"encoding/json"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/integration"
	"files/pkg/models"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

func (t *Task) DownloadFromFiles() error {
	var user = t.param.Owner
	var action = t.param.Action
	var src = t.param.Src
	var dst *models.FileParam

	if t.wasPaused {
		t.param.Dst = t.pausedParam
		t.pausedParam = nil
	}

	dst = t.param.Dst

	klog.Infof("[Task] Id: %s, start, downloadFormFiles, user: %s, action: %s, src: %s, dst: %s", t.id, user, action, common.ToJson(src), common.ToJson(dst))

	filesServer, err := integration.IntegrationManager().GetFilesPod(src.Extend)
	if err != nil {
		return err
	}

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     60 * time.Second,
		},
	}

	var srcFileName, isSrcFile = files.GetFileNameFromPath(src.Path)
	var srcFilePrefix, srcFileExt = common.SplitNameExt(srcFileName)
	var filesServerIp = filesServer.Status.PodIP

	filesServerUrl := fmt.Sprintf("http://%s/api/tree/%s/%s/%s", filesServerIp, src.FileType, src.Extend, strings.TrimPrefix(src.Path, "/"))
	filesListsReq, err := http.NewRequest("GET", filesServerUrl, nil)
	if err != nil {
		return err
	}
	filesListsReq.Header.Set(common.REQUEST_HEADER_OWNER, src.Owner)
	filesListsReq.Header.Set("Cache-Control", "no-cache")

	filesListsResp, err := client.Do(filesListsReq)
	if err != nil {
		return err
	}
	defer filesListsResp.Body.Close()

	if filesListsResp.StatusCode != http.StatusOK {
		return fmt.Errorf("request files list status invalid, %d", filesListsResp.StatusCode)
	}

	var result []files.FileInfo
	var totalFiles int
	var totalSize int64
	scanner := bufio.NewScanner(filesListsResp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.ReplaceAll(line, "\n", "")
		line = strings.ReplaceAll(line, "\r", "")
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		line = strings.TrimPrefix(line, "data: ")
		var fi files.FileInfo
		if e := json.Unmarshal([]byte(line), &fi); e != nil {
			klog.Errorf("[Task] Id: %s, serialize fileInfo error: %v, data: %s", t.id, err, line)
			return e
		}
		totalSize += fi.Size
		totalFiles += 1
		result = append(result, fi)
	}

	if err := scanner.Err(); err != nil {
		klog.Errorf("[Task] Id: %s, scan resp body error: %v", t.id, err)
		return err
	}

	if len(result) == 0 {
		klog.Infof("[Task] Id: %s, files not found, totalFiles: %d, totalSize: %d", t.id, totalFiles, totalSize)
		return nil
	}

	t.updateTotalSize(totalSize)

	dstUri, err := dst.GetResourceUri()
	if err != nil {
		klog.Errorf("[Task] Id: %s, dst uri found error: %v", t.id, err)
		return err
	}
	localSpace, err := common.CheckDiskSpace(dstUri, totalSize, dst.IsSystem()) // no sync dst but external here, IsSystem() is enough and precise
	if err != nil {
		return err
	}
	klog.Infof("[Task] Id: %s, check local free space, downloadSize: %s, downloadFiles: %d, localSpace: %s", t.id, common.FormatBytes(totalSize), totalFiles, common.FormatBytes(localSpace))

	dstPathPrefix := files.GetPrefixPath(dst.Path)

	dstPath := dstUri + dstPathPrefix

	if !t.wasPaused {
		dupNames, err := files.CollectDupNames(dstPath, srcFilePrefix, srcFileExt, isSrcFile)
		if err != nil {
			klog.Errorf("[Task] Id: %s, get dup name error: %v", t.id, err)
			return err
		}

		klog.Infof("[Task] Id: %s, srcName: %s, dstPath: %s, isfile: %v, dupName: %v", t.id, srcFileName, dstPath, isSrcFile, dupNames)

		newPrefixName := files.GenerateDupName(dupNames, srcFileName, isSrcFile)

		var newDstPath = dstPathPrefix + newPrefixName
		if !isSrcFile {
			if !strings.HasSuffix(newDstPath, "/") {
				newDstPath += "/"
			}
		}

		klog.Infof("[Task] Id: %s, new dst path: %s, new name: %s", t.id, newDstPath, newPrefixName)

		dst.Path = newDstPath
	}

	if ctxCancel, ctxErr := t.isCancel(); ctxCancel {
		t.pausedParam = dst
		t.pausedPhase = t.currentPhase
		return ctxErr
	}

	for _, fitem := range result {
		var cutSrcPath = strings.TrimPrefix(fitem.Path, src.Path)
		var filePath = path.Join(dstUri+dst.Path, cutSrcPath)
		var fileDir = path.Dir(filePath)
		if !files.FilePathExists(fileDir) {
			if err = files.MkdirAllWithChown(nil, fileDir, 0755, true, 1000, 1000); err != nil {
				klog.Errorf("[Task] Id: %s, mkdir %s error: %v", t.id, fileDir, err)
				break
			}
		}

		downloadUrl := fmt.Sprintf("http://%s/api/raw/%s/%s/%s", filesServerIp, src.FileType, src.Extend, url.PathEscape(strings.TrimPrefix(fitem.Path, "/")))

		klog.Infof("[Task] Id: %s, download file url %s, write file: %s", t.id, downloadUrl, filePath)

		var downloadReq *http.Request
		downloadReq, err = http.NewRequest("GET", downloadUrl, nil)
		if err != nil {
			klog.Errorf("[Task] Id: %s, download new request failed, url: %s, error: %v", t.id, downloadUrl, err)
			break
		}

		downloadReq.Header.Set(common.REQUEST_HEADER_OWNER, src.Owner)

		var downloadResp *http.Response

		downloadResp, err = client.Do(downloadReq)
		if err != nil {
			klog.Errorf("[Task] Id: %s, download failed, url: %s, error: %v", t.id, downloadUrl, err)
			downloadResp.Body.Close()
			break
		}

		if downloadResp.StatusCode != http.StatusOK {
			io.Copy(io.Discard, downloadResp.Body)
			downloadResp.Body.Close()
			klog.Errorf("[Task] Id: %s, download status invalid: %d", t.id, downloadResp.StatusCode)
			err = fmt.Errorf("download status invalid: %d", downloadResp.StatusCode)
			break
		}

		var fileWriter *os.File
		fileWriter, err = os.Create(filePath)
		if err != nil {
			klog.Errorf("[Task] Id: %s, create file writer error: %v, path: %s", t.id, err, filePath)
			io.Copy(io.Discard, downloadResp.Body)
			downloadResp.Body.Close()
			break
		}

		var buf = make([]byte, 256*1024)

		for {
			ctxCanceled, ctxErr := t.isCancel()
			if ctxCanceled {
				io.Copy(io.Discard, downloadResp.Body)
				err = ctxErr
				break
			}

			nr, er := downloadResp.Body.Read(buf)
			if nr > 0 {
				nw, ew := fileWriter.Write(buf[:nr])
				if ew != nil {
					klog.Errorf("[Task] Id: %s, write buffer error: %v", t.id, ew)
					err = ew
					break
				}

				if nw != nr {
					klog.Errorf("[Task] Id: %s, error short write", t.id)
					io.Copy(io.Discard, downloadResp.Body)
					err = io.ErrShortWrite
					break
				}

				var progress = (t.transfer * 100) / int64(totalSize)
				t.updateProgress(int(progress), int64(nw))
			}

			if er != nil {
				if er == io.EOF {
					break
				}

				klog.Errorf("[Task] Id: %s, read buffer error: %v", t.id, er)
				io.Copy(io.Discard, downloadResp.Body)
				err = er
				break
			}
		}

		downloadResp.Body.Close()
		fileWriter.Close()

		if err != nil {
			break
		}

		klog.Infof("[Task] Id: %s, download %s done!", t.id, filePath)
	}

	if err != nil {
		t.pausedParam = dst
		t.pausedPhase = t.currentPhase
		return err
	}

	klog.Infof("[Task] Id: %s done! phase: %d", t.id, t.currentPhase)

	return err
}
