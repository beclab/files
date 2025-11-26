package global

import (
	"encoding/json"
	"files/pkg/common"
	"files/pkg/files"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"k8s.io/klog/v2"
)

var (
	GlobalMounted *Mount
)

var externalWatcher *fsnotify.Watcher = nil

type Mount struct {
	Mounted map[string]*files.DiskInfo
	mu      sync.RWMutex
}

type MountedDevice struct {
	Code    int               `json:"code"`
	Data    []*files.DiskInfo `json:"data"`
	Message *string           `json:"message,omitempty"`
}

func init() {
	GlobalMounted = &Mount{
		Mounted: make(map[string]*files.DiskInfo),
	}
}

func InitGlobalMounted() {
	GlobalMounted.getMounted()
	GlobalMounted.watchMounted()
}

func (m *Mount) Updated() {
	GlobalMounted.getMounted()
}

func (m *Mount) GetMountedData() []files.DiskInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.Mounted == nil || len(m.Mounted) == 0 {
		return []files.DiskInfo{}
	}

	var res []files.DiskInfo
	for _, v := range m.Mounted {
		res = append(res, *v)
	}

	return res
}

func (m *Mount) watchMounted() {
	var err error
	if externalWatcher == nil {
		externalWatcher, err = fsnotify.NewWatcher()
		if err != nil {
			klog.Fatalf("Failed to initialize watcher: %v", err)
			panic(err)
		}
	}

	path := "/data/External"
	err = externalWatcher.Add(path)
	if err != nil {
		klog.Errorln("watcher add error:", err)
		panic(err)
	}
	klog.Infof("watcher initialized at: %s", path)

	go func() {
		for {
			select {
			case err, ok := <-externalWatcher.Errors:
				if !ok {
					klog.Errorf("watcher error channel closed")
					return
				}
				klog.Errorf("watcher error: %v", err)
			case e, ok := <-externalWatcher.Events:
				if !ok {
					klog.Warning("watcher event channel closed")
					return
				}

				klog.Infof("watcher event: %s, op: %s", e.Name, e.Op.String())
				if e.Has(fsnotify.Chmod) {
					continue
				}

				time.Sleep(300 * time.Millisecond)
				klog.Infof("mount watcher event: %s, op: %s", e.Name, e.Op.String())
				m.getMounted()
			}
		}
	}()
}

func (m *Mount) getMounted() {
	m.mu.Lock()
	defer m.mu.Unlock()

	var host = common.OlaresdHost

	if host == "" {
		klog.Errorf("olaresd host invalid, host: %s", host)
		return
	}

	// for 1.12: path-incluster URL exists, won't err in normal condition
	// for 1.11: path-incluster URL may not exist, if err, use usb-incluster and hdd-incluster for system functional
	url := "http://" + host + "/system/mounted-path-incluster"

	var header = make(map[string]string)
	header["X-Signature"] = "temp_signature"

	resp, err := common.Request(url, http.MethodGet, header, nil, false)
	if err != nil {
		klog.Errorf("get mounted error: %v", err)
		return
	}

	var result *MountedDevice
	if err := json.Unmarshal(resp, &result); err != nil {
		klog.Errorf("unmarshal mounted error: %v", err)
		return
	}

	if result.Code != 200 {
		klog.Errorf("get mounted invalid, message: %s", *result.Message)
		return
	}

	m.Mounted = make(map[string]*files.DiskInfo)

	if result.Data != nil {
		for _, d := range result.Data {
			m.Mounted[d.Path] = d
		}
	}

	klog.Infof("mounted device: %s", common.ToJson(result.Data))
}

func (m *Mount) CheckExternalType(path string, isDir bool) string {
	if path == "" {
		return "external"
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	if isDir && !strings.HasSuffix(path, "/") {
		path = path + "/"
	}

	var exists bool
	var mountType string
	for _, mount := range m.Mounted {
		if mount.Invalid {
			continue
		}
		if strings.HasPrefix(path, "/"+mount.Path+"/") {
			exists = true
			mountType = mount.Type
			break
		}
	}

	if !exists {
		mountType = "internal"
	}

	return mountType
}
