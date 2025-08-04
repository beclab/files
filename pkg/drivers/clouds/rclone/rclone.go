package rclone

import (
	"errors"
	"files/pkg/drivers/clouds/rclone/config"
	"files/pkg/drivers/clouds/rclone/operations"
	"files/pkg/drivers/clouds/rclone/serve"
	"files/pkg/utils"
	"fmt"
	"sync"

	"k8s.io/klog/v2"
)

var Command *rclone

var _ Interface = &rclone{}

type rclone struct {
	config    config.Interface
	serve     serve.Interface
	operation operations.Interface
	sync.RWMutex
}

func (r *rclone) GetConfig() config.Interface {
	return r.config
}

func (r *rclone) GetOperation() operations.Interface {
	return r.operation
}

func NewCommandRclone() {
	Command = &rclone{
		config:    config.NewConfig(),
		serve:     serve.NewServe(),
		operation: operations.NewOperations(),
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

	changedConfigs := r.checkChangedConfigs(configs)

	klog.Infof("[startHttp] changed configs: %s", utils.ToJson(changedConfigs))

	if len(changedConfigs.Delete) > 0 {
		for _, deleteServe := range changedConfigs.Delete {
			if err := r.stopServe(deleteServe.Name); err != nil {
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

func (r *rclone) restartServe(createConfig *config.Config) error {
	if err := r.stopServe(createConfig.Name); err != nil {
		klog.Errorf("restart serve, stop error: %v, name: %s", err, createConfig.Name)
	}
	if err := r.startServe(createConfig); err != nil {
		klog.Errorf("restart serve, start error: %v, name: %s", err, createConfig.Name)
	}
	return nil
}

func (r *rclone) startServe(createConfig *config.Config) error {
	if err := r.config.Create(createConfig); err != nil {
		return fmt.Errorf("create config, %v", err)
	}

	var targetPath string
	if createConfig.Type == "dropbox" {
		targetPath = createConfig.AccessKeyId
	} else {
		targetPath = createConfig.Bucket
	}
	_, err := r.serve.Start(createConfig.Name, targetPath)
	if err != nil {
		return fmt.Errorf("start serve, %v", err)
	}

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
			if k == createConfig.Name {
				found = true
				break
			}
		}
		if !found {
			changed.Delete = append(changed.Delete, v)
		}
	}

	for _, createConfig := range configs {
		if _, ok := serveConfigs[createConfig.Name]; !ok {
			changed.Create = append(changed.Create, createConfig)
		}
	}

	for _, createConfig := range configs {
		serveConfig, ok := serveConfigs[createConfig.Name]
		if !ok {
			continue
		}

		if !serveConfig.Equal(createConfig) {
			changed.Update = append(changed.Update, createConfig)
		}
	}

	return changed
}
