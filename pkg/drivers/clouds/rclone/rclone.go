package rclone

import (
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/clouds/rclone/config"
	"files/pkg/drivers/clouds/rclone/job"
	"files/pkg/drivers/clouds/rclone/operations"
	"files/pkg/drivers/clouds/rclone/serve"
	"files/pkg/files"
	"files/pkg/models"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"k8s.io/klog/v2"
)

var localConfig = &config.Config{
	ConfigName: common.Local,
	Name:       common.Local,
	Type:       common.Local,
}

var Command *rclone

var _ Interface = &rclone{}

type rclone struct {
	config    config.Interface
	serve     serve.Interface
	operation operations.Interface
	job       job.Interface
	sync.RWMutex
}

func (r *rclone) GetJob() job.Interface {
	return r.job
}

func (r *rclone) GetConfig() config.Interface {
	return r.config
}

func (r *rclone) GetOperation() operations.Interface {
	return r.operation
}

func (r *rclone) GetServe() serve.Interface {
	return r.serve
}

func NewCommandRclone() {
	Command = &rclone{
		config:    config.NewConfig(),
		serve:     serve.NewServe(),
		operation: operations.NewOperations(),
		job:       job.NewJob(),
	}
}

func (r *rclone) InitServes() {
	r.Lock()
	defer r.Unlock()

	configs, err := r.config.Dump()
	if err != nil {
		klog.Errorf("[loadHttp] load configs error: %v", err)
	}

	serves, err := r.serve.List()
	if err != nil {
		klog.Errorf("[loadHttp] load serves error: %v", err)
	}

	if configs != nil {
		r.config.SetConfigs(configs)
	}

	if serves != nil {
		r.serve.SetServes(serves)
	}

}

func (r *rclone) StartHttp(configs []*config.Config) error {
	r.Lock()
	defer r.Unlock()

	configs = append(configs, localConfig)

	changedConfigs := r.checkChangedConfigs(configs)

	changedConfigsJson := common.ToJson(changedConfigs)
	_ = changedConfigsJson

	// if changedConfigsJson != "{}" {
	klog.Infof("[startHttp] changed configs: %s", common.ToJson(changedConfigs))
	// }

	if len(changedConfigs.Delete) > 0 {
		for _, deleteServe := range changedConfigs.Delete {
			if err := r.stopServe(deleteServe.ConfigName); err != nil {
				klog.Errorf("[startHttp] stop serve, stop serve error: %v", err)
			}
			if err := r.deleteConfig(deleteServe.ConfigName); err != nil {
				klog.Errorf("[startHttp] stop serve, delete config error: %v", err)
			}
		}
	}

	if len(changedConfigs.Update) > 0 {
		for _, createConfig := range changedConfigs.Update {
			if err := r.restartServe(createConfig); err != nil {
				klog.Errorf("[startHttp] restart serve error: %v", err)
			}
		}
	}

	if len(changedConfigs.Create) > 0 {
		for _, createConfig := range changedConfigs.Create {
			if err := r.startServe(createConfig); err != nil {
				klog.Errorf("[startHttp] start serve error: %v", err)
			}
		}
	}

	return nil
}

// configName:bucket or configName:
func (r *rclone) GetFsPrefix(param *models.FileParam) (string, error) {
	switch param.FileType {
	case common.Drive, common.Cache, common.External:
		uri, err := param.GetResourceUri()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("local:%s", uri), nil
	case common.AwsS3, common.TencentCos:
		var configName = fmt.Sprintf("%s_%s_%s", param.Owner, param.FileType, param.Extend)
		config, err := r.config.GetConfig(configName)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s:%s", configName, config.Bucket), nil
	case common.DropBox, common.GoogleDrive:
		return fmt.Sprintf("%s_%s_%s:", param.Owner, param.FileType, param.Extend), nil
	}
	return "", errors.New("fs invalid")
}

func (r *rclone) restartServe(createConfig *config.Config) error {
	if err := r.stopServe(createConfig.ConfigName); err != nil {
		klog.Errorf("restart serve, stop error: %v, configName: %s", err, createConfig.ConfigName)
	}
	if err := r.startServe(createConfig); err != nil {
		klog.Errorf("restart serve, start error: %v, configName: %s", err, createConfig.ConfigName)
	}
	return nil
}

func (r *rclone) startServe(createConfig *config.Config) error {
	if err := r.config.Create(createConfig); err != nil {
		return fmt.Errorf("create config, %v", err)
	}

	var fsPath, err = r.config.GetFsPath(createConfig.ConfigName)
	if err != nil {
		return err
	}
	_, err = r.serve.Start(createConfig.ConfigName, fsPath)
	if err != nil {
		return fmt.Errorf("start serve, %v", err)
	}

	klog.Infof("start serve success, configName: %s", createConfig.ConfigName)

	return nil
}

func (r *rclone) deleteConfig(configName string) error {
	return r.config.Delete(configName)
}

func (r *rclone) stopServe(configName string) error {
	return r.serve.Stop(configName)
}

func (r *rclone) checkChangedConfigs(configs []*config.Config) *config.CreateConfigChanged {
	var changed = new(config.CreateConfigChanged)

	var serveConfigs = r.config.GetServeConfigs()
	for k, v := range serveConfigs {
		var found bool
		for _, createConfig := range configs {
			if k == createConfig.ConfigName {
				found = true
				break
			}
		}
		if !found {
			changed.Delete = append(changed.Delete, v)
		}
	}

	for _, createConfig := range configs {
		if _, ok := serveConfigs[createConfig.ConfigName]; !ok {
			changed.Create = append(changed.Create, createConfig)
		}
	}

	for _, createConfig := range configs {
		serveConfig, ok := serveConfigs[createConfig.ConfigName]
		if !ok {
			continue
		}

		if httpId := r.serve.GetHttpId(createConfig.ConfigName); httpId == "" {
			changed.Update = append(changed.Update, createConfig)
			continue
		}

		if !serveConfig.Equal(createConfig) {
			changed.Update = append(changed.Update, createConfig)
		}
	}

	return changed
}

func (r *rclone) GetSpaceSize(fileParam *models.FileParam) (int64, error) {
	var fsPrefix, err = r.GetFsPrefix(fileParam)
	if err != nil {
		return 0, err
	}

	var fs string
	fs = fsPrefix

	resp, err := r.operation.About(fs)
	if err != nil {
		return 0, err
	}

	return resp.Free, nil
}

func (r *rclone) GetFilesSize(fileParam *models.FileParam) (int64, error) {
	var fsPrefix, err = r.GetFsPrefix(fileParam)
	if err != nil {
		return 0, err
	}

	filesName, isFile := files.GetFileNameFromPath(fileParam.Path)
	prefixPath := files.GetPrefixPath(fileParam.Path)

	if !isFile {
		var fs string = fsPrefix + fileParam.Path
		resp, err := r.GetOperation().Size(fs)
		if err != nil {
			return 0, err
		}
		return resp.Bytes, nil
	}

	var fs = fsPrefix + prefixPath
	var remote = filesName
	var opts = &operations.OperationsOpt{
		FilesOnly: true,
	}
	resp, err := r.GetOperation().Stat(fs, remote, opts)
	if err != nil {
		return 0, err
	}

	return resp.Item.Size, nil

}

func (r *rclone) GetFilesList(param *models.FileParam, getPrefix bool) (*operations.OperationsList, error) {

	var fsPrefix, err = r.GetFsPrefix(param)
	if err != nil {
		return nil, err
	}
	var pathPrefix = files.GetPrefixPath(param.Path)
	var opt = &operations.OperationsOpt{
		NoModTime:  true,
		NoMimeType: true,
		Metadata:   false,
	}

	var fs string
	if getPrefix {
		fs = fsPrefix + pathPrefix
	} else {
		fs = fsPrefix + param.Path
	}

	lists, err := r.operation.List(fs, opt, nil)
	if err != nil {
		return nil, err
	}

	return lists, nil
}

func (r *rclone) CreateEmptyDirectory(param *models.FileParam) error {
	var localFs, localRemote string = fmt.Sprintf("%s:%s", common.Local, common.DefaultLocalRootPath), common.DefaultKeepFileName

	var dstFsPrefix, err = r.GetFsPrefix(param)
	if err != nil {
		return err
	}

	var dstFs, dstRemote string

	dstFs = dstFsPrefix
	dstRemote = param.Path

	if param.FileType == common.AwsS3 || param.FileType == common.TencentCos {
		dstRemote = strings.TrimPrefix(dstRemote, "/")
		if err = r.GetOperation().Copyfile(localFs, localRemote, dstFs, dstRemote); err != nil {
			return fmt.Errorf("fs: %s, remote: %s, error: %v", dstFs, dstRemote, err)
		}
	} else {
		dstRemote = strings.Trim(dstRemote, "/")
		if err = r.GetOperation().Mkdir(dstFs, dstRemote); err != nil {
			return fmt.Errorf("fs: %s, remote: %s, error: %v", dstFs, dstRemote, err)
		}
	}

	klog.Infof("[rclone] create empty dir done! <<< dstFs: %s, dstRemote: %s", dstFs, dstRemote)

	return nil
}

func (r *rclone) CreateEmptyDirectories(src, target *models.FileParam) error {
	var srcFsPrefix, err = r.GetFsPrefix(src)
	if err != nil {
		return err
	}
	dstFsPrefix, err := r.GetFsPrefix(target)
	if err != nil {
		return err
	}
	dstPrefix := files.GetPrefixPath(target.Path)
	dstName, _ := files.GetFileNameFromPath(target.Path)

	var srcOpt = &operations.OperationsOpt{
		Recurse:    true,
		DirsOnly:   true,
		NoModTime:  true,
		NoMimeType: true,
		Metadata:   false,
	}
	var srcFs = srcFsPrefix + src.Path
	srcDirItems, err := r.GetOperation().List(srcFs, srcOpt, nil)
	if err != nil {
		return err
	}
	var srcPathItems []string
	if srcDirItems != nil && srcDirItems.List != nil && len(srcDirItems.List) > 0 {
		for _, item := range srcDirItems.List {
			srcPathItems = append(srcPathItems, item.Path)
		}
	} else {
		srcPathItems = append(srcPathItems, "")
	}

	sort.Strings(srcPathItems)

	// tidy
	var srcPathFormated []string
	for i, path := range srcPathItems {
		isPrefix := false
		for j := i + 1; j < len(srcPathItems); j++ {
			if strings.HasPrefix(srcPathItems[j], path) &&
				len(srcPathItems[j]) > len(path) &&
				srcPathItems[j][len(path)] == '/' {
				isPrefix = true
				break
			}
		}
		if !isPrefix {
			srcPathFormated = append(srcPathFormated, path)
		}
	}

	klog.Infof("[rclone] get dirs: %v", srcPathFormated)

	// create
	var localFs, localRemote string = fmt.Sprintf("%s:%s", common.Local, common.DefaultLocalRootPath), common.DefaultKeepFileName
	var dstFs, dstRemote string = dstFsPrefix, ""

	for _, item := range srcPathFormated {
		dstRemote = dstPrefix + filepath.Join(dstName, item) + "/"

		if target.FileType == common.AwsS3 || target.FileType == common.TencentCos {
			dstRemote = strings.TrimPrefix(dstRemote, "/")
			if err = r.GetOperation().Copyfile(localFs, localRemote, dstFs, dstRemote); err != nil {
				return fmt.Errorf("copyfile failed, dstFs: %s, dstRemote: %s, error: %v", dstFs, dstRemote, err)
			}
		} else {
			dstRemote = strings.Trim(dstRemote, "/")
			if err = r.GetOperation().Mkdir(dstFs, dstRemote); err != nil {
				return fmt.Errorf("mkdir failed, dstFs: %s, dstRemote: %s, error: %v", dstFs, dstRemote, err)
			}
		}

		klog.Infof("[rclone] create empty dir done! <<< dstFs: %s, dstRemote: %s", dstFs, dstRemote)
	}

	return nil
}

// lock dst file or dir
func (r *rclone) CreatePlaceHolder(dst *models.FileParam) error {
	var localFs, localRemote = fmt.Sprintf("%s:%s", common.Local, common.DefaultLocalRootPath), common.DefaultKeepFileName

	dstFsPrefix, err := r.GetFsPrefix(dst)
	if err != nil {
		return err
	}

	dstPrefix := files.GetPrefixPath(dst.Path)
	dstName, isFile := files.GetFileNameFromPath(dst.Path)

	var dstFs, dstRemote string = dstFsPrefix, ""
	dstRemote = dstPrefix + dstName

	if isFile {
		dstRemote = strings.TrimPrefix(dstRemote, "/")
		if err = r.GetOperation().Copyfile(localFs, localRemote, dstFs, dstRemote); err != nil {
			return fmt.Errorf("copyfile failed, dstFs: %s, dstRemote: %s, error: %v", dstFs, dstRemote, err)
		}
	} else {
		dstRemote = strings.Trim(dstRemote, "/")
		if err = r.GetOperation().Mkdir(dstFs, dstRemote); err != nil {
			return fmt.Errorf("mkdir failed, dstFs: %s, dstRemote: %s, error: %v", dstFs, dstRemote, err)
		}
	}

	return nil
}

func (r *rclone) Copy(src, dst *models.FileParam) (*operations.OperationsAsyncJobResp, error) {
	_, isFile := files.GetFileNameFromPath(src.Path)
	srcFsPrefix, err := r.GetFsPrefix(src)
	if err != nil {
		return nil, fmt.Errorf("get src fs prefix failed, err: %v", err)
	}

	srcPrefix := files.GetPrefixPath(src.Path)
	srcFileName, _ := files.GetFileNameFromPath(src.Path)

	dstFsPrefix, err := r.GetFsPrefix(dst)
	if err != nil {
		return nil, fmt.Errorf("get dst fs prefix failed, err: %v", err)
	}

	dstPrefix := files.GetPrefixPath(dst.Path)
	dstFileName, _ := files.GetFileNameFromPath(dst.Path)

	var jobResp *operations.OperationsAsyncJobResp

	if isFile {

		// copy file
		var srcFs, srcRemote string
		var dstFs, dstRemote string

		srcFs = srcFsPrefix + srcPrefix
		srcRemote = srcFileName

		dstFs = dstFsPrefix + dstPrefix
		dstRemote = dstFileName

		klog.Infof("[rclone] copy file, srcFs: %s, srcRemote: %s, dstFs: %s, dstRemote: %s", srcFs, srcRemote, dstFs, dstRemote)

		jobResp, err = r.GetOperation().CopyfileAsync(srcFs, srcRemote, dstFs, dstRemote)
		if err != nil {
			return nil, fmt.Errorf("[rclone] copy file failed, srcFs: %s, srcR: %s, dstFs: %s, dstR: %s, error: %v", srcFs, srcRemote, dstFs, dstRemote, err)
		}

		klog.Infof("[rclone] cope file done! job: %d", *jobResp.JobId)

	} else {

		// copy dir
		srcFs := srcFsPrefix + src.Path
		dstFs := dstFsPrefix + dstPrefix + dstFileName + "/"

		klog.Infof("[rclone] copy dir, srcFs: %s, dstFs: %s", srcFs, dstFs)

		jobResp, err = r.GetOperation().CopyAsync(srcFs, dstFs)
		if err != nil {
			return nil, fmt.Errorf("[rclone] copy dir failed, srcFs: %s, dstFs: %s, error: %v", srcFs, dstFs, err)
		}

		klog.Infof("[rclone] copy dir done! job: %d", *jobResp.JobId)

	}

	if jobResp.JobId == nil {
		return nil, err // todo log
	}

	return jobResp, nil
}

func (r *rclone) ClearTaskCaches(param *models.FileParam, taskId string) error {
	var taskCachedPath = fmt.Sprintf("%s/%s", common.DefaultSyncUploadToCloudTempPath, taskId)

	if param.FileType != common.Cache {
		return fmt.Errorf("file type %s invalid", param.FileType)
	}

	srcUri, err := param.GetResourceUri()
	if err != nil {
		return err
	}
	var p = fmt.Sprintf("%s%s", srcUri, taskCachedPath)
	return os.RemoveAll(p)
}

func (r *rclone) Clear(param *models.FileParam) error {
	var err error
	var configName string
	var isSrcLocal bool
	var owner = param.Owner

	klog.Infof("[rclone] clear, param: %s", common.ToJson(param))
	fileName, isFile := files.GetFileNameFromPath(param.Path)
	prefixPath := files.GetPrefixPath(param.Path)

	if param.FileType == common.Drive || param.FileType == common.Cache || param.FileType == common.External {
		isSrcLocal = true
	}

	if isSrcLocal {
		configName = common.Local
	} else {
		configName = fmt.Sprintf("%s_%s_%s", param.Owner, param.FileType, param.Extend)
	}

	config, err := r.GetConfig().GetConfig(configName)
	if err != nil {
		return err
	}

	var fsPrefix string
	if isSrcLocal {
		srcUri, err := param.GetResourceUri()
		if err != nil {
			return err
		}
		fsPrefix = fmt.Sprintf("%s:%s", configName, srcUri)
	} else {
		fsPrefix = fmt.Sprintf("%s:%s", configName, config.Bucket)
	}

	if isFile {
		var fs, remote string
		fs = fsPrefix + prefixPath
		remote = fileName

		if err = r.GetOperation().Deletefile(fs, remote); err != nil {
			klog.Errorf("[rclone] clear, delete file error: %v, user: %s, isFile: %v, fs: %s, remote: %s", err, owner, isFile, fs, remote)
			return err
		}

		r.GetOperation().FsCacheClear()

		klog.Infof("[rclone] clear, file done! user: %s, fs: %s, remote: %s", owner, fs, remote)

		return nil
	}

	// purge
	var fs = fsPrefix + prefixPath
	var remote = fileName

	if err = r.GetOperation().Purge(fs, remote); err != nil {
		klog.Errorf("[rclone] clear, purge error: %v, user: %s, fs: %s, remote: %s", err, owner, fs, remote)
		return err
	}

	r.GetOperation().FsCacheClear()

	klog.Infof("[rclone] clear, purge done! user: %s, fs: %s, remote: %s", owner, fs, remote)

	return nil
}

func (r *rclone) Delete(param *models.FileParam, dirents []string) ([]string, error) {
	var user = param.Owner
	var deleteFailedPaths []string
	var total = len(dirents)

	for current, dp := range dirents {
		dp = strings.TrimSpace(dp) //  /path/ or /file

		dpd, err := url.PathUnescape(dp)
		if err != nil {
			klog.Errorf("[rclone] delete, path unescape error: %v, path: %s", err, dp)
			deleteFailedPaths = append(deleteFailedPaths, dp)
			continue
		}

		klog.Infof("[rclone] delete, delete (%d/%d), user: %s, file: %s", current+1, total, user, dpd)

		fsPrefix, err := r.GetFsPrefix(param)
		_, isFile := files.GetFileNameFromPath(dpd)

		var fs, remote string

		if isFile {
			fs = fsPrefix + param.Path
			remote = strings.TrimPrefix(dpd, "/")
			if err = r.GetOperation().Deletefile(fs, remote); err != nil {
				deleteFailedPaths = append(deleteFailedPaths, dp)
			}

		} else {
			fs = fsPrefix + param.Path
			remote = strings.Trim(dpd, "/")

			if err = r.GetOperation().Purge(fs, remote); err != nil {
				deleteFailedPaths = append(deleteFailedPaths, dp)
			}
		}
	}

	if len(deleteFailedPaths) > 0 {
		return deleteFailedPaths, fmt.Errorf("delete failed paths")
	}

	return nil, nil

}

func (r *rclone) StopJobs() error {
	klog.Infof("[rclone] stop running jobs")

	jobsResp, err := r.job.List()
	if err != nil {
		klog.Errorf("[rclone] get job list error: %v", err)
		return err
	}

	var jobList *job.JobListResp
	if err := json.Unmarshal(jobsResp, &jobList); err != nil {
		klog.Errorf("[rclone] unmarshal job list error: %v", err)
		return err
	}

	if jobList.JobIds == nil || len(jobList.JobIds) == 0 {
		return nil
	}

	klog.Infof("[rclone] running jobs: %v", jobList.JobIds)

	var stopIds []int
	for _, jobId := range jobList.JobIds {
		if _, err := r.job.Stop(jobId); err != nil {
			klog.Errorf("[rclone] stop job %d error: %v", jobId, err)
		} else {
			stopIds = append(stopIds, jobId)
		}
	}

	klog.Infof("[rclone] stop running jobs done! jobs: %v", stopIds)

	return nil
}

func (r *rclone) GetMatchedItems(fs string, opt *operations.OperationsOpt, filter *operations.OperationsFilter) (*operations.OperationsList, error) {
	// get matched file or dir exsits
	return r.GetOperation().List(fs, opt, filter)

}

func (r *rclone) FormatFilter(s string, fuzzy bool) []string {
	if s == "" {
		return nil
	}

	var l = ""
	if fuzzy {
		l = "*"
	}

	var result []string
	var c = s[0]
	if c == '*' {
		result = append(result, fmt.Sprintf("+ \\*%s%s/", s, l))
		result = append(result, fmt.Sprintf("+ /\\*%s%s", s, l))
	} else {
		result = append(result, fmt.Sprintf("+ %s%s/", s, l))
		result = append(result, fmt.Sprintf("+ /%s%s", s, l))
	}

	result = append(result, "- *")
	return result
}
