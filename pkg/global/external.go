package global

import (
	"encoding/json"
	"files/pkg/common"
	"files/pkg/files"
	"net/http"
	"path/filepath"
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
	// Mounted is the final merged view exposed to callers.
	Mounted map[string]*files.DiskInfo
	// polledMounted keeps the latest snapshot from olaresd pull.
	polledMounted map[string]*files.DiskInfo
	// reportedMounted keeps the latest snapshot from ReportMountedStates.
	reportedMounted map[string]*files.DiskInfo
	//Usage   float64
	//Free    uint64
	mu sync.RWMutex
}

type MountedDevice struct {
	Code    int               `json:"code"`
	Data    []*files.DiskInfo `json:"data"`
	Message *string           `json:"message,omitempty"`
}

// MountedPatch preserves field presence from /api/mounted_states payload.
// A nil pointer means the field was not provided by reporter.
type MountedPatch struct {
	Type              *string  `json:"type"`
	Path              *string  `json:"path"`
	Fstype            *string  `json:"fstype"`
	Total             *int64   `json:"total"`
	Free              *int64   `json:"free"`
	Used              *int64   `json:"used"`
	UsedPercent       *float64 `json:"usedPercent"`
	InodesTotal       *int64   `json:"inodesTotal"`
	InodesUsed        *int64   `json:"inodesUsed"`
	InodesFree        *int64   `json:"inodesFree"`
	InodesUsedPercent *float64 `json:"inodesUsedPercent"`
	ReadOnly          *bool    `json:"read_only"`
	Invalid           *bool    `json:"invalid"`
	IDSerial          *string  `json:"id_serial"`
	IDSerialShort     *string  `json:"id_serial_short"`
	PartitionUUID     *string  `json:"partition_uuid"`
}

func (p *MountedPatch) key() (string, bool) {
	if p == nil || p.Path == nil {
		return "", false
	}
	path := strings.TrimSpace(*p.Path)
	if path == "" {
		return "", false
	}
	return path, true
}

func (p *MountedPatch) applyTo(dst *files.DiskInfo) {
	if p == nil || dst == nil {
		return
	}
	if p.Type != nil {
		dst.Type = *p.Type
	}
	if p.Path != nil {
		dst.Path = *p.Path
	}
	if p.Fstype != nil {
		dst.Fstype = *p.Fstype
	}
	if p.Total != nil {
		dst.Total = *p.Total
	}
	if p.Free != nil {
		dst.Free = *p.Free
	}
	if p.Used != nil {
		dst.Used = *p.Used
	}
	if p.UsedPercent != nil {
		dst.UsedPercent = *p.UsedPercent
	}
	if p.InodesTotal != nil {
		dst.InodesTotal = *p.InodesTotal
	}
	if p.InodesUsed != nil {
		dst.InodesUsed = *p.InodesUsed
	}
	if p.InodesFree != nil {
		dst.InodesFree = *p.InodesFree
	}
	if p.InodesUsedPercent != nil {
		dst.InodesUsedPercent = *p.InodesUsedPercent
	}
	if p.ReadOnly != nil {
		readOnly := *p.ReadOnly
		dst.ReadOnly = &readOnly
	}
	if p.Invalid != nil {
		dst.Invalid = *p.Invalid
	}
	if p.IDSerial != nil {
		dst.IDSerial = *p.IDSerial
	}
	if p.IDSerialShort != nil {
		dst.IDSerialShort = *p.IDSerialShort
	}
	if p.PartitionUUID != nil {
		dst.PartitionUUID = *p.PartitionUUID
	}
}

func init() {
	GlobalMounted = &Mount{
		Mounted:         make(map[string]*files.DiskInfo),
		polledMounted:   make(map[string]*files.DiskInfo),
		reportedMounted: make(map[string]*files.DiskInfo),
	}
}

func cloneDiskInfo(d *files.DiskInfo) *files.DiskInfo {
	if d == nil {
		return nil
	}
	cloned := *d
	return &cloned
}

func buildMountedMap(disks []*files.DiskInfo) map[string]*files.DiskInfo {
	mounted := make(map[string]*files.DiskInfo, len(disks))
	for _, d := range disks {
		if d == nil || d.Path == "" {
			continue
		}
		mounted[d.Path] = cloneDiskInfo(d)
	}
	return mounted
}

func (m *Mount) mergeMountedLocked() {
	merged := make(map[string]*files.DiskInfo, len(m.polledMounted)+len(m.reportedMounted))
	for path, disk := range m.polledMounted {
		merged[path] = cloneDiskInfo(disk)
	}
	// Reported data has higher priority on conflict because it is
	// an explicit external status push.
	for path, disk := range m.reportedMounted {
		merged[path] = cloneDiskInfo(disk)
	}
	m.Mounted = merged
}

func (m *Mount) updatePolledMounted(disks []*files.DiskInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.polledMounted = buildMountedMap(disks)
	m.mergeMountedLocked()
}

func (m *Mount) UpdateReportedMounted(patches []*MountedPatch) {
	m.mu.Lock()
	defer m.mu.Unlock()
	reported := make(map[string]*files.DiskInfo, len(patches))
	for _, patch := range patches {
		path, ok := patch.key()
		if !ok {
			continue
		}
		var merged files.DiskInfo
		if base, exists := m.polledMounted[path]; exists && base != nil {
			merged = *base
		}
		patch.applyTo(&merged)
		merged.Path = path
		reported[path] = cloneDiskInfo(&merged)
	}
	m.reportedMounted = reported
	m.mergeMountedLocked()
}

// ClearReportedMounted drops all reporter-pushed overlay states so callers can
// immediately fall back to fresh polled mount data (for example right after an
// explicit mount/unmount operation).
func (m *Mount) ClearReportedMounted() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reportedMounted = make(map[string]*files.DiskInfo)
	m.mergeMountedLocked()
}

func InitGlobalMounted() {
	GlobalMounted.getMounted()
	GlobalMounted.watchMounted()
	//GlobalMounted.watchDiskUsage()
}

// ExternalWatcherClose stops the package-level fsnotify watcher used by
// watchMounted. Closing the watcher terminates its Errors/Events channels;
// the watcher goroutine selects on those and returns once they close.
// Safe to call when the watcher was never initialized.
func ExternalWatcherClose() error {
	if externalWatcher == nil {
		return nil
	}
	return externalWatcher.Close()
}

func (m *Mount) Updated() {
	GlobalMounted.getMounted()
}

func (m *Mount) GetMountedData() []files.DiskInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.Mounted) == 0 {
		return []files.DiskInfo{}
	}

	var res []files.DiskInfo
	for _, v := range m.Mounted {
		res = append(res, *v)
	}

	return res
}

// GetMountedByPath returns a mounted snapshot by root path name
// (for example "Samsung-0"). Safe for concurrent callers.
func (m *Mount) GetMountedByPath(path string) (*files.DiskInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if path == "" {
		return nil, false
	}
	d, ok := m.Mounted[path]
	if !ok || d == nil {
		return nil, false
	}
	cloned := *d
	return &cloned, true
}

// hasMount reports whether the mount map contains base under read lock.
// watchMounted previously read m.Mounted directly without the lock,
// racing with getMounted's full-map replacement and the cron-driven
// Updated() refresh. The Go runtime treats this as a map race and may
// crash with a fatal error.
func (m *Mount) hasMount(base string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.Mounted[base]
	return ok
}

// snapshotMountedJSON returns a JSON encoding of the current mount
// map taken under read lock. Used by watchMounted log lines that
// previously dumped m.Mounted directly while another goroutine could
// be replacing the map.
func (m *Mount) snapshotMountedJSON() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]*files.DiskInfo, len(m.Mounted))
	for k, v := range m.Mounted {
		out[k] = v
	}
	return common.ToJson(out)
}

//func (m *Mount) watchDiskUsage() {
//	go func() {
//		ticker := time.NewTicker(10 * time.Second)
//		defer ticker.Stop()
//
//		for range ticker.C {
//			usage, free, err := common.CheckDiskUsage()
//			if err != nil {
//				klog.Errorf("watch disk usage error: %v", err)
//				continue
//			}
//			m.Usage = usage
//			m.Free = free
//		}
//	}()
//}

func (m *Mount) watchMounted() {
	var err error
	if externalWatcher == nil {
		externalWatcher, err = fsnotify.NewWatcher()
		if err != nil {
			klog.Errorf("failed to initialize external mount watcher: %v; external mount notifications disabled", err)
			return
		}
	}

	path := "/data/External"
	err = externalWatcher.Add(path)
	if err != nil {
		klog.Errorf("external mount watcher add %q failed: %v; notifications disabled", path, err)
		return
	}
	klog.Infof("watcher initialized at: %s", path)

	go func() {
		maxRetries := 3
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

				klog.Infof("mount watcher event: %s, op: %s", e.Name, e.Op.String())
				if e.Op == fsnotify.Create {
					found := false
					m.getMounted()
					base := filepath.Base(e.Name)
					if m.hasMount(base) {
						found = true
						klog.Infof("Found %s in mounted disks (immediate), mounted: %s", e.Name, m.snapshotMountedJSON())
					} else {
						retryDelay := 1 * time.Second
						for i := 0; i < maxRetries; i++ {
							time.Sleep(retryDelay)
							klog.Infof("Retry %d for %s (wait %v)", i+1, e.Name, retryDelay)

							m.getMounted()
							if m.hasMount(base) {
								found = true
								klog.Infof("Found %s in mounted disks after %d retries, mounted: %s", e.Name, i+1, m.snapshotMountedJSON())
								break
							}
							retryDelay *= 2
						}
					}

					if !found {
						klog.Warningf("Failed to find %s in mounted disks after %d attempts, mounted: %s", e.Name, maxRetries, m.snapshotMountedJSON())
					}
				} else {
					m.getMounted()
				}
			}
		}
	}()
}

func (m *Mount) getMounted() {
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
		if result.Message != nil {
			klog.Errorf("get mounted invalid, message: %s", *result.Message)
		} else {
			klog.Errorf("get mounted invalid, code: %d", result.Code)
		}
		return
	}

	m.updatePolledMounted(result.Data)

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
