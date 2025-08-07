package operations

import (
	"context"
	"encoding/json"
	"files/pkg/drivers/clouds/rclone/common"
	"files/pkg/drivers/clouds/rclone/config"
	"files/pkg/drivers/clouds/rclone/utils"
	commonutils "files/pkg/utils"
	"fmt"
	"net/http"

	"k8s.io/klog/v2"
)

type Interface interface {
	List(fs string, opts *OperationsOpt) (*OperationsList, error)
	Stat(fs string, remote string, opts *OperationsOpt) (*OperationsStat, error)
	Mkdir(fs string, dirName string) error
	Uploadfile(fs string, dirName string) error
	Copyfile(srcFs string, srcR string, dstFs string, dstR string, async *bool) (*OperationsCopyFileResp, error)
	MoveFile(srcFs string, srcR string, dstFs string, dstR string, async *bool) (*OperationsCopyFileResp, error)
	Deletefile(fs string, remote string) error
	DeleteDir(fs string, remote string) error
	DeleteDirs(fs string, remote string, leaveRoot bool) error
	Delete(fs string) error
	Purge(fs string, remote string) error

	Size(fs string) (*OperationsSizeResp, error)

	Copy(srcFs, dstFs string, async *bool) (*OperationsCopyFileResp, error) // copy a directory,no suit for files
	Move(srcFs, dstFs string) (*OperationsCopyFileResp, error)              // move a directory, no suit for files
}

type operations struct {
	config config.Interface
}

func NewOperations() *operations {
	return &operations{}
}

func (o *operations) Size(fs string) (*OperationsSizeResp, error) {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, SizePath)

	var param = OperationsReq{
		Fs: fs,
	}

	resp, err := utils.Request(context.Background(), url, http.MethodPost, nil, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations size error: %v, fs: %s", err, fs)
		return nil, err
	}

	var data *OperationsSizeResp
	if err := json.Unmarshal(resp, &data); err != nil {
		return nil, err
	}

	klog.Infof("[rclone] operations size done, resp: %s, fs: %s", string(resp), fs)

	return data, nil
}

func (o *operations) Stat(fs string, remote string, opts *OperationsOpt) (*OperationsStat, error) {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, StatPath)

	var param = OperationsReq{
		Fs:     fs,     // xxx:yyy
		Remote: remote, // folder/  or  file
		Opt:    opts,
	}

	klog.Infof("[rclone] operations stat, param: %s", commonutils.ToJson(param))

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

	klog.Infof("[rclone] operations stat done, fs: %s, data: %s", fs, commonutils.ToJson(data))

	return data, nil
}

func (o *operations) Copyfile(srcFs string, srcR string, dstFs string, dstR string, async *bool) (*OperationsCopyFileResp, error) {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, CopyfilePath)
	var param = OperationsReq{
		SrcFs:     srcFs,
		SrcRemote: srcR,
		DstFs:     dstFs,
		DstRemote: dstR,
	}

	if async != nil {
		param.Async = async
	}

	klog.Infof("[rclone] operations copyfile, data: %s", commonutils.ToJson(param))

	resp, err := utils.Request(context.Background(), url, http.MethodPost, nil, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations mkdir error: %v", err)
		return nil, err
	}

	var job *OperationsCopyFileResp
	if err := json.Unmarshal(resp, &job); err != nil {
		return nil, err
	}

	klog.Infof("[rclone] operations copyfile success, resp: %s", commonutils.ToJson(job))

	return job, nil
}

func (o *operations) MoveFile(srcFs string, srcR string, dstFs string, dstR string, async *bool) (*OperationsCopyFileResp, error) {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, Movefilepath)
	var param = OperationsReq{
		SrcFs:     srcFs,
		SrcRemote: srcR,
		DstFs:     dstFs,
		DstRemote: dstR,
	}

	if async != nil {
		param.Async = async
	}

	klog.Infof("[rclone] operations movefile, data: %s", commonutils.ToJson(param))

	resp, err := utils.Request(context.Background(), url, http.MethodPost, nil, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations movefile error: %v", err)
		return nil, err
	}

	var job *OperationsCopyFileResp
	if err := json.Unmarshal(resp, &job); err != nil {
		return nil, err
	}

	klog.Infof("[rclone] operations movefile success, resp: %s", commonutils.ToJson(job))

	return job, nil
}

func (o *operations) Uploadfile(fs string, dirName string) error {
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

func (o *operations) List(fs string, opts *OperationsOpt) (*OperationsList, error) {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, ListPath)
	var param = OperationsReq{
		Fs:     fs,
		Remote: "",
		Opt:    opts,
	}

	klog.Infof("[rclone] operations list param: %s", commonutils.ToJson(param))

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

func (o *operations) Copy(srcFs, dstFs string, async *bool) (*OperationsCopyFileResp, error) {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, SyncCopyPath)
	var param = SyncCopyReq{
		SrcFs:              srcFs,
		DstFs:              dstFs,
		CreateEmptySrcDirs: true,
	}
	if async != nil {
		param.Async = async
	}

	klog.Infof("[rclone] operations copy, srcFs: %s, dstFs: %s", srcFs, dstFs)

	resp, err := utils.Request(context.Background(), url, http.MethodPost, nil, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations copy error: %v", err)
		return nil, err
	}

	var job *OperationsCopyFileResp
	if err := json.Unmarshal(resp, &job); err != nil {
		return nil, err
	}

	klog.Infof("[rclone] operations copy success, resp: %s", commonutils.ToJson(job))

	return job, nil
}

func (o *operations) Move(srcFs, dstFs string) (*OperationsCopyFileResp, error) {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, SyncMovePath)
	var param = SyncCopyReq{
		SrcFs:              srcFs,
		DstFs:              dstFs,
		CreateEmptySrcDirs: true,
		DeleteEmptySrcDirs: true,
	}

	klog.Infof("[rclone] operations move, srcFs: %s, dstFs: %s, param: %s", srcFs, dstFs, commonutils.ToJson(param))

	resp, err := utils.Request(context.Background(), url, http.MethodPost, nil, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations move error: %v", err)
		return nil, err
	}

	var job *OperationsCopyFileResp
	if err := json.Unmarshal(resp, &job); err != nil {
		return nil, err
	}

	klog.Infof("[rclone] operations move success, resp: %s", commonutils.ToJson(job))

	return job, nil

}

func (o *operations) Deletefile(fs string, remote string) error {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, DeletefilePath)

	var param = OperationsReq{
		Fs:     fs,
		Remote: remote,
	}

	klog.Infof("[rclone] operations deletefile, param: %s", commonutils.ToJson(param))

	resp, err := utils.Request(context.Background(), url, http.MethodPost, nil, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations deletefile error: %v, fs: %s, remote: %s", err, fs, remote)
		return err
	}

	klog.Infof("[rclone] operations deletefile done, resp: %s, fs: %s, remote: %s", string(resp), fs, remote)

	return nil
}

func (o *operations) Delete(fs string) error {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, DeletePath)

	var param = OperationsReq{
		Fs: fs,
	}

	klog.Infof("[rclone] operations delete, param: %s", commonutils.ToJson(param))

	resp, err := utils.Request(context.Background(), url, http.MethodPost, nil, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations delete error: %v, fs: %s", err, fs)
		return err
	}

	klog.Infof("[rclone] operations delete done, resp: %s, fs: %s", string(resp), fs)

	return nil
}

func (o *operations) DeleteDir(fs string, remote string) error {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, DeletedirPath)

	var param = OperationsReq{
		Fs:     fs,
		Remote: remote,
	}

	klog.Infof("[rclone] operations rmdir, param: %s", commonutils.ToJson(param))

	resp, err := utils.Request(context.Background(), url, http.MethodPost, nil, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations rmdir error: %v, fs: %s, remote: %s", err, fs, remote)
		return err
	}

	klog.Infof("[rclone] operations rmdir done, resp: %s, fs: %s, remote: %s", string(resp), fs, remote)

	return nil
}

func (o *operations) DeleteDirs(fs string, remote string, leaveRoot bool) error {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, DeletedirsPath)

	var param = OperationsReq{
		Fs:        fs,
		Remote:    remote,
		LeaveRoot: &leaveRoot,
	}

	klog.Infof("[rclone] operations rmdirs, param: %s", commonutils.ToJson(param))

	resp, err := utils.Request(context.Background(), url, http.MethodPost, nil, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations rmdirs error: %v, fs: %s, remote: %s", err, fs, remote)
		return err
	}

	klog.Infof("[rclone] operations rmdirs done, resp: %s, fs: %s, remote: %s", string(resp), fs, remote)

	return nil
}

func (o *operations) Purge(fs string, remote string) error {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, PurgePath)

	var param = OperationsReq{
		Fs:     fs,     // xxx:yyy/parent
		Remote: remote, // dir/
	}

	klog.Infof("[rclone] operations purge, param: %s", commonutils.ToJson(param))

	resp, err := utils.Request(context.Background(), url, http.MethodPost, nil, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations purge error: %v, fs: %s, remote: %s", err, fs, remote)
		return err
	}

	klog.Infof("[rclone] operations purge done, resp: %s, fs: %s, remote: %s", string(resp), fs, remote)

	return nil
}
