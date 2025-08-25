package operations

import (
	"context"
	"encoding/json"
	commonutils "files/pkg/common"
	"files/pkg/drivers/clouds/rclone/common"
	"files/pkg/drivers/clouds/rclone/config"
	"files/pkg/drivers/clouds/rclone/utils"
	"fmt"
	"net/http"

	"k8s.io/klog/v2"
)

type Interface interface {
	List(fs string, opts *OperationsOpt) (*OperationsList, error)
	Stat(fs string, remote string, opts *OperationsOpt) (*OperationsStat, error)
	About(fs string) (*OperationsAboutResp, error)
	Mkdir(fs string, dirName string) error
	Uploadfile(fs string, dirName string) error
	Copyfile(srcFs string, srcR string, dstFs string, dstR string) error
	MoveFile(srcFs string, srcR string, dstFs string, dstR string) error

	Copy(srcFs, dstFs string) error // copy a directory,no suit for files
	Move(srcFs, dstFs string) error // move a directory, no suit for files

	Deletefile(fs string, remote string) error
	Purge(fs string, remote string) error

	Size(fs string) (*OperationsSizeResp, error)

	CopyfileAsync(srcFs string, srcR string, dstFs string, dstR string) (*OperationsAsyncJobResp, error)
	MovefileAsync(srcFs string, srcR string, dstFs string, dstR string) (*OperationsAsyncJobResp, error)
	CopyAsync(srcFs, dstFs string) (*OperationsAsyncJobResp, error) // copy a directory,no suit for files
	MoveAsync(srcFs, dstFs string) (*OperationsAsyncJobResp, error) // move a directory, no suit for files

	FsCacheClear() error
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
	// Give information about the supplied file or directory
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

	klog.Infof("[rclone] operations stat done! fs: %s, data: %s", fs, commonutils.ToJson(data))

	return data, nil
}

func (o *operations) About(fs string) (*OperationsAboutResp, error) {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, AboutPath)

	var param = OperationsReq{
		Fs: fs, // xxx:yyy
	}

	klog.Infof("[rclone] operations about, param: %s", commonutils.ToJson(param))

	resp, err := utils.Request(context.Background(), url, http.MethodPost, nil, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations about error: %v, fs: %s", err, fs)
		return nil, err
	}

	var data *OperationsAboutResp
	if err := json.Unmarshal(resp, &data); err != nil {
		klog.Errorf("[rclone] operations about unmarshal error: %v, fs: %s", err, fs)
		return nil, err
	}

	klog.Infof("[rclone] operations about done! fs: %s, data: %s", fs, commonutils.ToJson(data))

	return data, nil
}

func (o *operations) Copyfile(srcFs string, srcR string, dstFs string, dstR string) error {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, CopyfilePath)
	var async = false
	var param = OperationsReq{
		SrcFs:     srcFs,
		SrcRemote: srcR,
		DstFs:     dstFs,
		DstRemote: dstR,
		Async:     &async,
	}

	klog.Infof("[rclone] operations copyfile, data: %s", commonutils.ToJson(param))

	resp, err := utils.Request(context.Background(), url, http.MethodPost, nil, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations mkdir error: %v", err)
		return err
	}

	klog.Infof("[rclone] operations copyfile done! resp: %s", string(resp))

	return nil
}

func (o *operations) MoveFile(srcFs string, srcR string, dstFs string, dstR string) error {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, Movefilepath)
	var async = false
	var param = OperationsReq{
		SrcFs:     srcFs,
		SrcRemote: srcR,
		DstFs:     dstFs,
		DstRemote: dstR,
		Async:     &async,
	}

	klog.Infof("[rclone] operations movefile, data: %s", commonutils.ToJson(param))

	resp, err := utils.Request(context.Background(), url, http.MethodPost, nil, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations movefile error: %v", err)
		return err
	}

	klog.Infof("[rclone] operations movefile done! resp: %s", string(resp))

	return nil
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

	klog.Infof("[rclone] operations mkdir done! fs: %s, remote: %s, resp: %s", fs, dirName, string(resp))

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

func (o *operations) Copy(srcFs, dstFs string) error {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, SyncCopyPath)
	var async = false
	var param = SyncCopyReq{
		SrcFs:              srcFs,
		DstFs:              dstFs,
		CreateEmptySrcDirs: true,
		Async:              &async,
	}

	klog.Infof("[rclone] operations copy, srcFs: %s, dstFs: %s", srcFs, dstFs)

	resp, err := utils.Request(context.Background(), url, http.MethodPost, nil, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations copy error: %v", err)
		return err
	}

	klog.Infof("[rclone] operations copy done! resp: %s", string(resp))

	return nil
}

func (o *operations) Move(srcFs, dstFs string) error {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, SyncMovePath)
	var async = false
	var param = SyncCopyReq{
		SrcFs:              srcFs,
		DstFs:              dstFs,
		CreateEmptySrcDirs: true,
		DeleteEmptySrcDirs: true,
		Async:              &async,
	}

	klog.Infof("[rclone] operations move, srcFs: %s, dstFs: %s, param: %s", srcFs, dstFs, commonutils.ToJson(param))

	resp, err := utils.Request(context.Background(), url, http.MethodPost, nil, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations move error: %v", err)
		return err
	}

	klog.Infof("[rclone] operations move done! resp: %s", string(resp))

	return nil

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

	klog.Infof("[rclone] operations deletefile done! resp: %s, fs: %s, remote: %s", string(resp), fs, remote)

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

func (o *operations) FsCacheClear() error {
	// var url = fmt.Sprintf("%s/%s", common.ServeAddr, FsCacheClearPath)

	// klog.Info("[rclone] operations fscacheClear")

	// var header = make(http.Header)
	// header.Add("Content-Type", "application/octet-stream")

	// _, err := utils.Request(context.Background(), url, http.MethodPost, &header, nil)
	// if err != nil {
	// 	klog.Errorf("[rclone] operations fscacheClear error: %v", err)
	// 	return err
	// }

	// klog.Info("[rclone] operations fscacheClear done!")

	return nil
}

func (o *operations) CopyfileAsync(srcFs string, srcR string, dstFs string, dstR string) (*OperationsAsyncJobResp, error) {
	var async = true
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, CopyfilePath)
	var param = OperationsReq{
		SrcFs:     srcFs,
		SrcRemote: srcR,
		DstFs:     dstFs,
		DstRemote: dstR,
		Async:     &async,
	}

	klog.Infof("[rclone] operations copyfileasync, data: %s", commonutils.ToJson(param))

	resp, err := utils.Request(context.Background(), url, http.MethodPost, nil, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations copyfileasync error: %v", err)
		return nil, err
	}

	var job *OperationsAsyncJobResp
	if err := json.Unmarshal(resp, &job); err != nil {
		return nil, err
	}

	klog.Infof("[rclone] operations copyfileasync done! resp: %s", commonutils.ToJson(job))

	return job, nil
}

func (o *operations) MovefileAsync(srcFs string, srcR string, dstFs string, dstR string) (*OperationsAsyncJobResp, error) {
	var async = true
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, Movefilepath)
	var param = OperationsReq{
		SrcFs:     srcFs,
		SrcRemote: srcR,
		DstFs:     dstFs,
		DstRemote: dstR,
		Async:     &async,
	}

	klog.Infof("[rclone] operations movefileasync, data: %s", commonutils.ToJson(param))

	resp, err := utils.Request(context.Background(), url, http.MethodPost, nil, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations movefileasync error: %v", err)
		return nil, err
	}

	var job *OperationsAsyncJobResp
	if err := json.Unmarshal(resp, &job); err != nil {
		return nil, err
	}

	klog.Infof("[rclone] operations movefileasync done! resp: %s", commonutils.ToJson(job))

	return job, nil
}

func (o *operations) CopyAsync(srcFs, dstFs string) (*OperationsAsyncJobResp, error) {
	var async = true
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, SyncCopyPath)
	var param = SyncCopyReq{
		SrcFs:              srcFs,
		DstFs:              dstFs,
		CreateEmptySrcDirs: true,
		Async:              &async,
	}

	klog.Infof("[rclone] operations copyasync, srcFs: %s, dstFs: %s", srcFs, dstFs)

	resp, err := utils.Request(context.Background(), url, http.MethodPost, nil, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations copyasync error: %v", err)
		return nil, err
	}

	var job *OperationsAsyncJobResp
	if err := json.Unmarshal(resp, &job); err != nil {
		return nil, err
	}

	klog.Infof("[rclone] operations copyasync done! resp: %s", commonutils.ToJson(job))

	return job, nil
}

func (o *operations) MoveAsync(srcFs, dstFs string) (*OperationsAsyncJobResp, error) {
	var async = true
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, SyncMovePath)
	var param = SyncCopyReq{
		SrcFs:              srcFs,
		DstFs:              dstFs,
		CreateEmptySrcDirs: true,
		DeleteEmptySrcDirs: true,
		Async:              &async,
	}

	klog.Infof("[rclone] operations moveasync, srcFs: %s, dstFs: %s, param: %s", srcFs, dstFs, commonutils.ToJson(param))

	resp, err := utils.Request(context.Background(), url, http.MethodPost, nil, []byte(commonutils.ToJson(param)))
	if err != nil {
		klog.Errorf("[rclone] operations moveasync error: %v", err)
		return nil, err
	}

	var job *OperationsAsyncJobResp
	if err := json.Unmarshal(resp, &job); err != nil {
		return nil, err
	}

	klog.Infof("[rclone] operations moveasync done! resp: %s", commonutils.ToJson(job))

	return job, nil
}
