package operations

import (
	"bytes"
	"context"
	"encoding/json"
	"files/pkg/drivers/clouds/rclone/common"
	"files/pkg/drivers/clouds/rclone/config"
	"files/pkg/drivers/clouds/rclone/utils"
	commonutils "files/pkg/utils"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"

	"k8s.io/klog/v2"
)

type Interface interface {
	List(fs string) (*OperationsList, error)
	Stat(fs string, remote string) (*OperationsStat, error)
	Mkdir(fs string, dirName string) error
	Uploadfile(fs string, dirName string) error
	Copyfile(srcFs string, srcR string, dstFs string, dstR string) error
}

type operations struct {
	config config.Interface
}

func NewOperations() *operations {
	return &operations{}
}

func (o *operations) Stat(fs string, remote string) (*OperationsStat, error) {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, StatPath)

	var param = OperationsReq{
		Fs:     fs,
		Remote: remote,
		Opt: &OperationsOpt{
			Metadata: true,
		},
	}

	resp, err := utils.Request(context.Background(), url, http.MethodPost, nil, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations stat error: %v, fs: %s", err, fs)
		return nil, err
	}

	var data *OperationsStat
	if err := json.Unmarshal(resp, &data); err != nil {
		klog.Errorf("[rclone] operations stat unmarshal error: %v, fs: %s", err, fs)
		return nil, err
	}

	klog.Infof("[rclone] operations stat done, fs: %s", fs)

	return data, nil
}

func (o *operations) Copyfile(srcFs string, srcR string, dstFs string, dstR string) error {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, CopyfilePath)
	var param = OperationsReq{
		SrcFs:     srcFs,
		SrcRemote: srcR,
		DstFs:     dstFs,
		DstRemote: dstR,
	}

	klog.Infof("[rclone] operations copyfile, data: %s", commonutils.ToJson(param))

	resp, err := utils.Request(context.Background(), url, http.MethodPost, nil, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations mkdir error: %v", err)
		return err
	}

	klog.Infof("[rclone] operations copyfile success, resp: %s", string(resp))

	return nil
}

func (o *operations) Uploadfile(fs string, dirName string) error {
	var url = fmt.Sprintf("%s/%s?fs=%s&remote=%s", common.ServeAddr, UploadfilePath, url.QueryEscape(fs), url.QueryEscape(dirName))
	// var param = OperationsReq{
	// 	Fs:     fs,
	// 	Remote: dirName,
	// 	Source: "/root/.keep",
	// }
	payload := &bytes.Buffer{}
	file, errFile1 := os.Open("/root/.keep")
	defer file.Close()
	writer := multipart.NewWriter(payload)
	part1, errFile1 := writer.CreateFormFile("File", ".")
	_, errFile1 = io.Copy(part1, file)
	if errFile1 != nil {
		return errFile1
	}
	err := writer.Close()
	if err != nil {
		return err
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/octet-stream")
	headers.Set("Content-Type", writer.FormDataContentType())

	resp, err := utils.Request(context.Background(), url, http.MethodPost, &headers, []byte(payload.String()))
	if err != nil {
		klog.Errorf("[rclone] operations uploadfile error: %v, fs: %s", err, fs)
		return err
	}

	klog.Infof("[rclone] operations uploadfile success fs: %s, remote: %s, resp: %s", fs, dirName, string(resp))

	return nil
}

func (o *operations) Mkdir(fs string, dirName string) error {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, MkdirPath)
	var param = OperationsReq{
		Fs:     fs,
		Remote: dirName,
	}

	resp, err := utils.Request(context.Background(), url, http.MethodPost, nil, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations mkdir error: %v, fs: %s", err, fs)
		return err
	}

	klog.Infof("[rclone] operations mkdir success fs: %s, remote: %s, resp: %s", fs, dirName, string(resp))

	return nil
}

func (o *operations) List(fs string) (*OperationsList, error) {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, ListPath)
	var param = OperationsReq{
		Fs:     fs,
		Remote: "",
		Opt: &OperationsOpt{
			Metadata: true,
		},
	}

	resp, err := utils.Request(context.Background(), url, http.MethodPost, nil, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations list error: %v, fs: %s", err, fs)
		return nil, err
	}

	var data *OperationsList
	if err := json.Unmarshal(resp, &data); err != nil {
		klog.Errorf("[rclone] operations list unmarshal error: %v, fs: %s", err, fs)
		return nil, err
	}

	klog.Infof("[rclone] operations list done, fs: %s", fs)

	return data, nil
}
