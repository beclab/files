package http

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type Awss3ListParam struct {
	Path  string `json:"path"`
	Drive string `json:"drive"`
	Name  string `json:"name"`
}

type Awss3ListResponse struct {
	StatusCode string                       `json:"status_code"`
	FailReason *string                      `json:"fail_reason,omitempty"`
	Data       []*Awss3ListResponseFileData `json:"data"`
	sync.Mutex
}

type Awss3ListResponseFileData struct {
	Path      string                     `json:"path"`
	Name      string                     `json:"name"`
	Size      int64                      `json:"size"`
	FileSize  int64                      `json:"fileSize"`
	Extension string                     `json:"extension"`
	Modified  *string                    `json:"modified,omitempty"`
	Mode      string                     `json:"mode"`
	IsDir     bool                       `json:"isDir"`
	IsSymlink bool                       `json:"isSymlink"`
	Type      string                     `json:"type"`
	Meta      *Awss3ListResponseFileMeta `json:"meta,omitempty"`
}

type Awss3ListResponseFileMeta struct {
	ETag         string  `json:"e_tag"`
	Key          string  `json:"key"`
	LastModified *string `json:"last_modified,omitempty"`
	Owner        *string `json:"owner,omitempty"`
	Size         int     `json:"size"`
	StorageClass string  `json:"storage_class"`
}

type Awss3MetaResponseMeta struct {
	ETag         string  `json:"e_tag"`
	Key          string  `json:"key"`
	LastModified *string `json:"last_modified,omitempty"`
	Owner        *string `json:"owner"`
	Size         int64   `json:"size"`
	StorageClass *string `json:"storage_class"`
}

type Awss3MetaResponseData struct {
	Path      string                `json:"path"`
	Name      string                `json:"name"`
	Size      int64                 `json:"size"`
	FileSize  int64                 `json:"fileSize"`
	Extension string                `json:"extension"`
	Modified  *string               `json:"modified,omitempty"`
	Mode      string                `json:"mode"`
	IsDir     bool                  `json:"isDir"`
	IsSymlink bool                  `json:"isSymlink"`
	Type      string                `json:"type"`
	Meta      Awss3MetaResponseMeta `json:"meta"`
}

type Awss3MetaResponse struct {
	StatusCode string                `json:"status_code"`
	FailReason *string               `json:"fail_reason"`
	Data       Awss3MetaResponseData `json:"data"`
}

type Awss3FocusedMetaInfos struct {
	Key   string `json:"key"`
	Path  string `json:"path"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"is_dir"`
}

type Awss3PostParam struct {
	ParentPath string `json:"parent_path"`
	FolderName string `json:"folder_name"`
	Drive      string `json:"drive"`
	Name       string `json:"name"`
}

type Awss3PostResponseFileMeta struct {
	Capabilities                 *bool       `json:"capabilities,omitempty"`
	CopyRequiresWriterPermission *bool       `json:"copyRequiresWriterPermission,omitempty"`
	CreatedTime                  *time.Time  `json:"createdTime,omitempty"`
	ExplicitlyTrashed            *bool       `json:"explicitlyTrashed,omitempty"`
	FileExtension                *string     `json:"fileExtension,omitempty"`
	FullFileExtension            *string     `json:"fullFileExtension,omitempty"`
	HasThumbnail                 *bool       `json:"hasThumbnail,omitempty"`
	HeadRevisionId               *string     `json:"headRevisionId,omitempty"`
	IconLink                     *string     `json:"iconLink,omitempty"`
	ID                           string      `json:"id"`
	IsAppAuthorized              *bool       `json:"isAppAuthorized,omitempty"`
	Kind                         string      `json:"kind"`
	LastModifyingUser            *struct{}   `json:"lastModifyingUser,omitempty"`
	LinkShareMetadata            *struct{}   `json:"linkShareMetadata,omitempty"`
	MD5Checksum                  *string     `json:"md5Checksum,omitempty"`
	MimeType                     string      `json:"mimeType"`
	ModifiedByMe                 *bool       `json:"modifiedByMe,omitempty"`
	ModifiedTime                 *time.Time  `json:"modifiedTime,omitempty"`
	Name                         string      `json:"name"`
	OriginalFilename             *string     `json:"originalFilename,omitempty"`
	OwnedByMe                    *bool       `json:"ownedByMe,omitempty"`
	Owners                       []*struct{} `json:"owners,omitempty"`
	QuotaBytesUsed               *int64      `json:"quotaBytesUsed,omitempty"`
	SHA1Checksum                 *string     `json:"sha1Checksum,omitempty"`
	SHA256Checksum               *string     `json:"sha256Checksum,omitempty"`
	Shared                       *bool       `json:"shared,omitempty"`
	SharedWithMeTime             *time.Time  `json:"sharedWithMeTime,omitempty"`
	Size                         *int64      `json:"size,omitempty"`
	Spaces                       *string     `json:"spaces,omitempty"`
	Starred                      *bool       `json:"starred,omitempty"`
	ThumbnailLink                *string     `json:"thumbnailLink,omitempty"`
	ThumbnailVersion             *int64      `json:"thumbnailVersion,omitempty"`
	Title                        *string     `json:"title,omitempty"`
	Trashed                      *bool       `json:"trashed,omitempty"`
	Version                      *int64      `json:"version,omitempty"`
	ViewedByMe                   *bool       `json:"viewedByMe,omitempty"`
	ViewedByMeTime               *time.Time  `json:"viewedByMeTime,omitempty"`
	ViewersCanCopyContent        *bool       `json:"viewersCanCopyContent,omitempty"`
	WebContentLink               *string     `json:"webContentLink,omitempty"`
	WebViewLink                  *string     `json:"webViewLink,omitempty"`
	WritersCanShare              *bool       `json:"writersCanShare,omitempty"`
}

type Awss3PostResponseFileData struct {
	Extension string                          `json:"extension"`
	FileSize  int64                           `json:"fileSize"`
	IsDir     bool                            `json:"isDir"`
	IsSymlink bool                            `json:"isSymlink"`
	Meta      GoogleDrivePostResponseFileMeta `json:"meta"`
	Mode      string                          `json:"mode"`
	Modified  string                          `json:"modified"`
	Name      string                          `json:"name"`
	Path      string                          `json:"path"`
	Size      int64                           `json:"size"`
	Type      string                          `json:"type"`
}

type Awss3PostResponse struct {
	Data       GoogleDrivePostResponseFileData `json:"data"`
	FailReason *string                         `json:"fail_reason,omitempty"`
	StatusCode string                          `json:"status_code"`
}

type Awss3DownloadFileSyncParam struct {
	LocalFolder   string `json:"local_folder"`
	CloudFilePath string `json:"cloud_file_path"`
	Drive         string `json:"drive"`
	Name          string `json:"name"`
}

func getAwss3FocusedMetaInfos(src string, w http.ResponseWriter, r *http.Request) (info *Awss3FocusedMetaInfos, err error) {
	src = strings.TrimSuffix(src, "/")
	info = nil
	err = nil

	srcDrive, srcName, srcPath := parseAwss3Path(src)

	param := Awss3ListParam{
		Path:  srcPath,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	// 将数据序列化为 JSON
	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}
	fmt.Println("Awss3 Awss3MetaResponseMeta Params:", string(jsonBody))
	respBody, err := Awss3Call("/drive/get_file_meta_data", "POST", jsonBody, w, r, true)
	if err != nil {
		fmt.Println("Error calling drive/get_file_meta_data:", err)
		return
	}

	var bodyJson Awss3MetaResponse
	if err = json.Unmarshal(respBody, &bodyJson); err != nil {
		fmt.Println(err)
		return
	}

	info = &Awss3FocusedMetaInfos{
		Key:   bodyJson.Data.Meta.Key,
		Path:  bodyJson.Data.Path,
		Name:  bodyJson.Data.Name,
		Size:  bodyJson.Data.FileSize,
		IsDir: bodyJson.Data.IsDir,
	}
	return
}

func generateAwss3FilesData(body []byte, stopChan <-chan struct{}, dataChan chan<- string,
	w http.ResponseWriter, r *http.Request, param Awss3ListParam) {
	defer close(dataChan)

	var bodyJson Awss3ListResponse
	if err := json.Unmarshal(body, &bodyJson); err != nil {
		fmt.Println(err)
		return
	}

	var A []*Awss3ListResponseFileData
	bodyJson.Lock()
	A = append(A, bodyJson.Data...)
	bodyJson.Unlock()

	for len(A) > 0 {
		fmt.Println("len(A): ", len(A))
		firstItem := A[0]
		fmt.Println("firstItem Path: ", firstItem.Path)
		fmt.Println("firstItem Name:", firstItem.Name)

		if firstItem.IsDir {
			firstParam := Awss3ListParam{
				Path:  firstItem.Path,
				Drive: param.Drive,
				Name:  param.Name,
			}
			firstJsonBody, err := json.Marshal(firstParam)
			if err != nil {
				fmt.Println("Error marshalling JSON:", err)
				fmt.Println(err)
				return
			}
			var firstRespBody []byte
			firstRespBody, err = Awss3Call("/drive/ls", "POST", firstJsonBody, w, r, true)

			var firstBodyJson Awss3ListResponse
			if err := json.Unmarshal(firstRespBody, &firstBodyJson); err != nil {
				fmt.Println(err)
				return
			}

			A = append(firstBodyJson.Data, A[1:]...)
		} else {
			dataChan <- formatSSEvent(firstItem)

			A = A[1:]
		}

		select {
		case <-stopChan:
			return
		default:
		}
	}
}

func streamAwss3Files(w http.ResponseWriter, r *http.Request, body []byte, param Awss3ListParam) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	stopChan := make(chan struct{})
	dataChan := make(chan string)

	go generateAwss3FilesData(body, stopChan, dataChan, w, r, param)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	for {
		select {
		case event, ok := <-dataChan:
			if !ok {
				return
			}
			_, err := w.Write([]byte(event))
			if err != nil {
				fmt.Println(err)
				return
			}
			flusher.Flush()

		case <-r.Context().Done():
			close(stopChan)
			return
		}
	}
}

func awss3FileToBuffer(src, bufferFilePath string, w http.ResponseWriter, r *http.Request) error {
	src = strings.TrimSuffix(src, "/")
	if !strings.HasSuffix(bufferFilePath, "/") {
		bufferFilePath += "/"
	}
	srcDrive, srcName, srcPath := parseAwss3Path(src)
	//srcPathId, srcDrive, srcName, srcDir, srcFilename, err := GoogleDrivePathToId(src, w, r, false)
	fmt.Println("srcDrive:", srcDrive, "srcName:", srcName, "srcPath:", srcPath)
	if srcPath == "" {
		fmt.Println("Src parse failed.")
		return nil
	}

	// 填充数据
	param := Awss3DownloadFileSyncParam{
		LocalFolder:   bufferFilePath,
		CloudFilePath: srcPath,
		Drive:         srcDrive, // "my_drive",
		Name:          srcName,  // "file_name",
	}

	// 将数据序列化为 JSON
	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return err
	}
	fmt.Println("Download File Params:", string(jsonBody))

	//var respBody []byte
	_, err = Awss3Call("/drive/download_sync", "POST", jsonBody, w, r, true)
	if err != nil {
		fmt.Println("Error calling drive/download_sync:", err)
		return err
	}
	return nil
	//var respJson GoogleDriveTaskResponse
	//if err = json.Unmarshal(respBody, &respJson); err != nil {
	//	fmt.Println(err)
	//	return err
	//}
	//taskId := respJson.Data.ID
	//taskParam := GoogleDriveTaskQueryParam{
	//	TaskIds: []string{taskId},
	//}
	//// 将数据序列化为 JSON
	//taskJsonBody, err := json.Marshal(taskParam)
	//if err != nil {
	//	fmt.Println("Error marshalling JSON:", err)
	//	return err
	//}
	//fmt.Println("Task Params:", string(taskJsonBody))
	//
	//for {
	//	time.Sleep(1000 * time.Millisecond)
	//	var taskRespBody []byte
	//	taskRespBody, err = GoogleDriveCall("/drive/task/query/task_ids", "POST", taskJsonBody, w, r, true)
	//	if err != nil {
	//		fmt.Println("Error calling drive/download_async:", err)
	//		return err
	//	}
	//	var taskRespJson GoogleDriveTaskQueryResponse
	//	if err = json.Unmarshal(taskRespBody, &taskRespJson); err != nil {
	//		fmt.Println(err)
	//		return err
	//	}
	//	if len(taskRespJson.Data) == 0 {
	//		return e.New("Task Info Not Found")
	//	}
	//	if taskRespJson.Data[0].Status != "Waiting" && taskRespJson.Data[0].Status != "InProgress" {
	//		if taskRespJson.Data[0].Status == "Completed" {
	//			return nil
	//		}
	//		return e.New(taskRespJson.Data[0].Status)
	//	}
	//}
}

func Awss3Call(dst, method string, reqBodyJson []byte, w http.ResponseWriter, r *http.Request, returnResp bool) ([]byte, error) {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return nil, os.ErrPermission
	}

	authority := r.Header.Get("Authority")
	fmt.Println("*****Awss3 Call URL authority:", authority)
	host := r.Header.Get("Origin")
	if host == "" {
		host = getHost(w, r) // r.Header.Get("Origin")
	}
	fmt.Println("*****Awss3 Call URL host:", host)
	dstUrl := host + dst // "/api/resources%2FHome%2FDocuments%2F"

	fmt.Println("dstUrl:", dstUrl)

	var req *http.Request
	var err error
	if reqBodyJson != nil {
		req, err = http.NewRequest(method, dstUrl, bytes.NewBuffer(reqBodyJson))
	} else {
		req, err = http.NewRequest(method, dstUrl, nil)
	}

	if err != nil {
		fmt.Println("Error creating request:", err)
		return nil, err
	}

	// 设置请求头
	req.Header = r.Header.Clone()
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return nil, err
	}
	defer resp.Body.Close()

	// 检查Content-Type
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		fmt.Println("Awss3 Call Response is not JSON format:", contentType)
	}

	// 读取响应体
	var body []byte
	if resp.Header.Get("Content-Encoding") == "gzip" {
		// 如果响应体被gzip压缩
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Println("Error creating gzip reader:", err)
			return nil, err
		}
		defer reader.Close()

		body, err = ioutil.ReadAll(reader)
		if err != nil {
			fmt.Println("Error reading gzipped response body:", err)
			return nil, err
		}
	} else {
		// 如果响应体没有被压缩
		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error reading response body:", err)
			return nil, err
		}
	}

	// 解析JSON
	var datas map[string]interface{}
	err = json.Unmarshal(body, &datas)
	if err != nil {
		fmt.Println("Error unmarshaling JSON response:", err)
		return nil, err
	}

	// 打印解析后的数据
	fmt.Println("Parsed JSON response:", datas)
	// 将解析后的JSON响应体转换为字符串（格式化输出）
	responseText, err := json.MarshalIndent(datas, "", "  ")
	if err != nil {
		http.Error(w, "Error marshaling JSON response to text: "+err.Error(), http.StatusInternalServerError)
		return nil, err
	}

	if returnResp {
		return responseText, nil
	}
	// 设置响应头并写入响应体
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write([]byte(responseText))
	return nil, nil
}

func parseAwss3Path(src string) (drive, name, path string) {
	if strings.HasPrefix(src, "/Drive/awss3") {
		src = src[12:]
		drive = "awss3"
	}

	slashes := []int{}
	for i, char := range src {
		if char == '/' {
			slashes = append(slashes, i)
		}
	}

	if len(slashes) < 2 {
		fmt.Println("Path does not contain enough slashes.")
		return drive, "", ""
	}

	name = src[1:slashes[1]]
	path = src[slashes[1]:]
	return drive, name, path
}

func resourceGetAwss3(w http.ResponseWriter, r *http.Request, stream int, meta int) (int, error) {
	src := r.URL.Path
	fmt.Println("src Path:", src)

	srcDrive, srcName, srcPath := parseAwss3Path(src)
	fmt.Println("srcDrive: ", srcDrive, ", srcName: ", srcName, ", src Path: ", srcPath)

	param := Awss3ListParam{
		Path:  srcPath,
		Drive: srcDrive, // "my_drive",
		Name:  srcName,  // "file_name",
	}

	// 将数据序列化为 JSON
	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return errToStatus(err), err
	}
	fmt.Println("Awss3 List Params:", string(jsonBody))
	if stream == 1 {
		var body []byte
		body, err = Awss3Call("/drive/ls", "POST", jsonBody, w, r, true)
		streamAwss3Files(w, r, body, param)
		return 0, nil
	}
	if meta == 1 {
		_, err = Awss3Call("/drive/get_file_meta_data", "POST", jsonBody, w, r, false)
	} else {
		_, err = Awss3Call("/drive/ls", "POST", jsonBody, w, r, false)
	}
	if err != nil {
		fmt.Println("Error calling drive/ls:", err)
		return errToStatus(err), err
	}
	return 0, nil
}

func splitPath(path string) (dir, name string) {
	// 去掉结尾的"/"
	trimmedPath := strings.TrimRight(path, "/")

	// 查找最后一个"/"的位置
	lastIndex := strings.LastIndex(trimmedPath, "/")

	if lastIndex == -1 {
		// 如果没有找到"/"，则dir为"/"，name为整个trimmedPath
		return "/", trimmedPath
	}

	// 分割dir和name，注意这里dir不包括最后的"/"
	dir = trimmedPath[:lastIndex+1] // 包括到最后一个"/"之前的部分
	// 如果路径只有根目录和"/"，则name应为空
	if lastIndex+1 == len(trimmedPath) {
		name = ""
	} else {
		name = trimmedPath[lastIndex+1:]
	}

	// 如果dir只有一个"/"，则表示根目录
	if dir == "/" {
		// 特殊处理根目录情况，此时name应为整个trimmedPath
		name = strings.TrimPrefix(trimmedPath, "/")
		dir = "/"
	}

	return dir, name
}

func resourcePostAwss3(src string, w http.ResponseWriter, r *http.Request, returnResp bool) ([]byte, int, error) {
	if src == "" {
		src = r.URL.Path
	}
	fmt.Println("src Path:", src)

	srcDrive, srcName, srcPath := parseAwss3Path(src)
	fmt.Println("srcDrive: ", srcDrive, ", srcName: ", srcName, ", src Path: ", srcPath)
	path, newName := splitPath(srcPath)

	param := Awss3PostParam{
		ParentPath: path,
		FolderName: newName,
		Drive:      srcDrive, // "my_drive",
		Name:       srcName,  // "file_name",
	}

	jsonBody, err := json.Marshal(param)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return nil, errToStatus(err), err
	}
	fmt.Println("Awss3 Post Params:", string(jsonBody))
	var respBody []byte = nil
	if returnResp {
		respBody, err = Awss3Call("/drive/create_folder", "POST", jsonBody, w, r, true)
	} else {
		_, err = Awss3Call("/drive/create_folder", "POST", jsonBody, w, r, false)
	}
	if err != nil {
		fmt.Println("Error calling drive/create_folder:", err)
		return respBody, errToStatus(err), err
	}
	return respBody, 0, nil
}
