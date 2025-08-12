package serve

import (
	"context"
	"encoding/json"
	"errors"
	"files/pkg/drivers/clouds/rclone/common"
	"files/pkg/drivers/clouds/rclone/utils"
	commonutils "files/pkg/utils"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"k8s.io/klog/v2"
)

type Interface interface {
	Get(configName string) *ServeResp
	Start(configName, configPath string) (string, error)
	Stop(configName string) error
	GetHttpId(configName string) string
	List() (map[string]*Serve, error)
	SetServes(serves map[string]*Serve)
}

type serve struct {
	https map[string]*Serve // {owner}_{fileType}_{access_key}
}

type ServeResp struct {
	Reader io.ReadCloser
	Error  error
}

var _ Interface = &serve{}

func NewServe() *serve {
	return &serve{
		https: make(map[string]*Serve),
	}
}

func (s *serve) SetServes(serves map[string]*Serve) {
	s.https = serves
}

func (s *serve) Get(configName string) *ServeResp {
	val, ok := s.https[configName]
	if !ok {
		return nil
	}

	var result = new(ServeResp)
	var addr = val.Addr
	reader, err := utils.Get(context.Background(), addr, nil, nil)
	if err != nil {
		result.Error = err
		return result
	}

	result.Reader = reader
	return result
}

func (s *serve) Start(configName, configPath string) (string, error) {
	var ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	port := s.generateServePort()

	addr := fmt.Sprintf("%s:%d", common.ServeHost, port)
	var startServe = &Serve{
		Name:         configName,
		Type:         "http",
		Fs:           fmt.Sprintf("%s:%s", configName, configPath),
		Addr:         addr,
		VfsCacheMode: "off",
		BufferSize:   "2Mi",
		Port:         port,
	}

	var host = fmt.Sprintf("%s/%s", common.ServeAddr, StartServePath)
	resp, err := utils.Request(ctx, host, http.MethodPost, nil, commonutils.ToBytes(startServe))

	if err != nil {
		return "", fmt.Errorf("%v, configName: %s, addr: %s", err, configName, addr)
	}

	var respHttp *StartServeResp
	if err := json.Unmarshal(resp, &respHttp); err != nil {
		return "", err
	}

	startServe.Id = respHttp.Id

	s.https[configName] = startServe

	klog.Infof("[rclone] start serve success, configName: %s, addr: %s", configName, addr)

	return respHttp.Id, nil
}

func (s *serve) Stop(configName string) error {
	var ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var httpId = s.GetHttpId(configName)
	if httpId == "" {
		return fmt.Errorf("http not found, configName: %s", configName)
	}
	var host = fmt.Sprintf("%s/%s", common.ServeAddr, StopServePath)
	var data = map[string]string{
		"id": httpId,
	}
	if _, err := utils.Request(ctx, host, http.MethodPost, nil, commonutils.ToBytes(data)); err != nil {
		klog.Errorf("[rclone] stop serve error: %v, httpId: %s, configName: %s", err, httpId, configName)
	}

	delete(s.https, configName)

	return nil
}

func (s *serve) List() (map[string]*Serve, error) {
	var ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var host = fmt.Sprintf("%s/%s", common.ServeAddr, ListServePath)
	resp, err := utils.Request(ctx, host, http.MethodPost, nil, []byte("{}"))
	if err != nil {
		klog.Errorf("[rclone] serve list error: %v", err)
		return nil, err
	}

	var servesResp *ServeListResp
	if err := json.Unmarshal(resp, &servesResp); err != nil {
		klog.Errorf("[rclone] serve list unmarshal error: %v", err)
		return nil, err
	}

	var serves = make(map[string]*Serve)
	if len(servesResp.List) > 0 {
		for _, serve := range servesResp.List {
			var sd = &Serve{
				Id:           serve.Id,
				Name:         serve.Params.Name,
				Type:         serve.Params.Type,
				Fs:           serve.Params.Fs,
				Addr:         serve.Params.Addr,
				BufferSize:   serve.Params.BufferSize,
				VfsCacheMode: serve.Params.VfsCachemode,
				Port:         serve.Params.Port,
			}
			serves[serve.Params.Name] = sd
		}
	}

	klog.Infof("[rclone] serve dump http list: %d", len(serves))

	return serves, nil
}

func (s *serve) GetHttpId(configName string) string {
	var httpId string
	for k, v := range s.https {
		if k == configName {
			httpId = v.Id
			break
		}
	}

	return httpId
}

func (s *serve) generateServePort() int {
	if len(s.https) == 0 {
		return ServeStartPort
	}

	var ports []int

	for _, serve := range s.https {
		ports = append(ports, serve.Port)
	}

	sort.Ints(ports)

	var port, err = s.getPort(ports)
	if err != nil {
		return ServeStartPort
	}
	return port + 1
}

func (s *serve) getPort(ports []int) (int, error) {
	if len(ports) == 0 {
		return 0, errors.New("ports not exists")
	}

	var found bool
	var port int
	for i := 1; i < len(ports); i++ {
		if ports[i] != ports[i-1]+1 {
			found = true
			port = ports[i-1]
			break
		}
	}
	if !found {
		port = ports[len(ports)-1]
	}
	return port, nil
}
