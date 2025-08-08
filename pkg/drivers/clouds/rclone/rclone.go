package rclone

import (
	"errors"
	"files/pkg/drivers/clouds/rclone/config"
	"files/pkg/drivers/clouds/rclone/job"
	"files/pkg/drivers/clouds/rclone/operations"
	"files/pkg/drivers/clouds/rclone/serve"
	"files/pkg/models"
	"files/pkg/utils"
	"fmt"
	"path/filepath"
	"sync"

	"k8s.io/klog/v2"
)

var localConfig = &config.Config{
	ConfigName: "local",
	Name:       "local",
	Type:       "local",
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

	klog.Infof("[startHttp] changed configs: %s", utils.ToJson(changedConfigs))

	if len(changedConfigs.Delete) > 0 {
		for _, deleteServe := range changedConfigs.Delete {
			if err := r.stopServe(deleteServe.ConfigName); err != nil {
				klog.Errorf("[startHttp] stop serve error: %v", err)
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

	klog.Infof("[startHttp] complete, print begin =====")
	r.config.Dump()
	r.serve.List()
	klog.Infof("[startHttp] complete, print end   =====")

	return nil
}

func (r *rclone) FormatFs(param *models.FileParam) (string, error) { // format  dir
	switch param.FileType {
	case utils.Drive, utils.Cache, utils.External:
		uri, err := param.GetResourceUri()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("local:%s", filepath.Join(uri, filepath.Dir(param.Path))), nil
	case utils.Sync:
		return "", errors.New("sync not support")
	case utils.AwsS3, utils.TencentCos:
		var configName = fmt.Sprintf("%s_%s_%s", param.Owner, param.FileType, param.Extend)
		config, err := r.config.GetConfig(configName)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s:%s", configName, filepath.Join(config.Bucket, filepath.Dir(param.Path))), nil
	case utils.DropBox, utils.GoogleDrive:
		return fmt.Sprintf("%s_%s_%s:%s", param.Owner, param.FileType, param.Extend, filepath.Dir(param.Path)), nil
	}
	return "", errors.New("fs invalid")
}

func (r *rclone) FormatRemote(param *models.FileParam) (string, error) { // format  file
	return filepath.Base(param.Path), nil
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

func (r *rclone) stopServe(configName string) error {
	var wg sync.WaitGroup
	var errCh = make(chan error, 2)

	wg.Add(2)

	go func() {
		defer wg.Done()
		err := r.serve.Stop(configName)
		errCh <- err
	}()

	go func() {
		defer wg.Done()
		err := r.config.Delete(configName)
		errCh <- err
	}()

	wg.Wait()
	close(errCh)

	var errList []error
	for err := range errCh {
		if err != nil {
			errList = append(errList, err)
		}
	}

	if len(errList) == 0 {
		return nil
	}

	return fmt.Errorf("%v, stop serve", errors.Join(errList...))
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
