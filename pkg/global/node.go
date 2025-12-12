package global

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	NodeGVR = schema.GroupVersionResource{
		Group: "", Version: "v1", Resource: "nodes",
	}
)

var (
	GlobalNode      *Node
	CurrentNodeName string
)

type NodeInfo struct {
	Name   string `json:"name"`
	Master bool   `json:"master"`
}

type Node struct {
	k8sClient *kubernetes.Clientset
	Nodes     map[string]*v1.Node
	mu        sync.RWMutex
}

func InitGlobalNodes(config *rest.Config) error {
	CurrentNodeName = os.Getenv("NODE_NAME")

	GlobalNode = &Node{
		k8sClient: kubernetes.NewForConfigOrDie(config),
		Nodes:     make(map[string]*v1.Node),
	}

	if err := GlobalNode.getGlobalNodes(); err != nil {
		return err
	}

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			GlobalNode.getGlobalNodes()
		}
	}()

	return nil
}

func (g *Node) IsMasterNode(nodeName string) bool {
	// todo check node annotation
	return true
}

func (g *Node) CheckNodeExists(nodeName string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	_, ok := g.Nodes[nodeName]
	return ok
}

func (g *Node) GetNodes() []NodeInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var nodes []NodeInfo
	for _, n := range g.Nodes {
		l := n.Labels
		_, isMaster := l["node-role.kubernetes.io/control-plane"]

		var node = NodeInfo{
			Name:   n.Name,
			Master: isMaster,
		}
		nodes = append(nodes, node)
	}

	return nodes
}

func (g *Node) GetNodeIp(nodeName string) string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, n := range g.Nodes {
		if nodeName == n.Name {
			if len(n.Status.Addresses) > 0 {
				for _, addr := range n.Status.Addresses {
					if addr.Type == "InternalIP" {
						return addr.Address
					}
				}
			}
		}
	}
	return ""
}

func (g *Node) GetMasterNode() string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var master string

	for _, n := range g.Nodes {
		l := n.Labels
		_, isMaster := l["node-role.kubernetes.io/control-plane"]
		if isMaster {
			master = n.Name
			break
		}
	}

	return master
}

func (g *Node) getGlobalNodes() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	var ctx, cancel = context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	config := ctrl.GetConfigOrDie()
	client := kubernetes.NewForConfigOrDie(config)

	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Errorf("list nodes error: %v", err)
		return err
	}

	for _, node := range nodes.Items {
		g.Nodes[node.Name] = &node
	}

	return nil
}

func (g *Node) Handlerevent() cache.ResourceEventHandler {
	return cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			return true
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				GlobalNode.getGlobalNodes()
			},
			DeleteFunc: func(obj interface{}) {
				GlobalNode.getGlobalNodes()
			},
		},
	}
}

type NodeDetailedInfo struct {
	Name           string
	Labels         map[string]string
	Conditions     []Condition
	Capacity       ResourceInfo
	Allocatable    ResourceInfo
	UsedStorage    ResourceInfo
	FreeStorage    ResourceInfo
	StorageMetrics StorageMetrics
}

type Condition struct {
	Type          corev1.NodeConditionType
	Status        corev1.ConditionStatus
	LastHeartbeat string
	Reason        string
	Message       string
}

type ResourceInfo struct {
	CPU              string
	Memory           string
	EphemeralStorage string
	Storage          string
}

type StorageMetrics struct {
	TotalCapacity   resource.Quantity
	UsedCapacity    resource.Quantity
	FreeCapacity    resource.Quantity
	UsagePercentage float64
}

func (g *Node) CheckDiskPressure() (bool, error) {
	node, exists := g.Nodes[CurrentNodeName]
	if !exists {
		klog.Infof("Get node info failed")
		return false, fmt.Errorf("get node info failed") // TODO
	}

	info := NodeDetailedInfo{
		Name:   node.Name,
		Labels: node.Labels,
	}

	// 收集节点条件
	for _, cond := range node.Status.Conditions {
		info.Conditions = append(info.Conditions, Condition{
			Type:          cond.Type,
			Status:        cond.Status,
			LastHeartbeat: cond.LastHeartbeatTime.String(),
			Reason:        cond.Reason,
			Message:       cond.Message,
		})
	}

	// 获取资源信息
	info.Capacity = getResourceInfo(node.Status.Capacity)
	info.Allocatable = getResourceInfo(node.Status.Allocatable)
	info.UsedStorage = calculateUsedResources(node.Status.Capacity, node.Status.Allocatable)
	info.FreeStorage = calculateFreeResources(node.Status.Capacity, node.Status.Allocatable)

	// 计算存储指标
	if cap, exists := node.Status.Capacity[corev1.ResourceEphemeralStorage]; exists {
		info.StorageMetrics = calculateStorageMetrics(cap, info.UsedStorage.EphemeralStorage)
	}

	// 格式化输出
	jsonOutput, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("JSON序列化失败: %v", err))
	}

	klog.Infof("节点详细信息:\n%s\n", jsonOutput)
	klog.Infof("====================================")

	var capacity resource.Quantity
	if capacity, exists = node.Status.Capacity[corev1.ResourceEphemeralStorage]; exists {
		klog.Infof("  All storage space: %s bytes\n", capacity.String())
	}

	if allocatable, exists := node.Status.Allocatable[corev1.ResourceEphemeralStorage]; exists {
		klog.Infof("  Available storage space: %s bytes\n", allocatable.String())

		used := capacity.DeepCopy()
		used.Sub(allocatable)
		usedPercent := float64(used.Value()) / float64(capacity.Value()) * 100
		klog.Infof("  Storage use percentage: %.2f%%\n", usedPercent)
	}
	klog.Infof("====================================")

	for _, cond := range node.Status.Conditions {
		klog.Infof("Node %s condition: %+v\n", CurrentNodeName, cond)
		if cond.Type == corev1.NodeDiskPressure && cond.Status == corev1.ConditionTrue {
			klog.Infof("Disk pressure detected! Node %s is suffering disk pressure\n", CurrentNodeName)
			//executeDefenseActions()
			return true, nil
		}
	}

	klog.Infof("No disk pressure detected")
	return false, nil
}

func getResourceInfo(resourceList corev1.ResourceList) ResourceInfo {
	return ResourceInfo{
		CPU:              getResourceValue(resourceList, corev1.ResourceCPU),
		Memory:           getResourceValue(resourceList, corev1.ResourceMemory),
		EphemeralStorage: getResourceValue(resourceList, corev1.ResourceEphemeralStorage),
		Storage:          getResourceValue(resourceList, corev1.ResourceStorage),
	}
}

func getResourceValue(resourceList corev1.ResourceList, resource corev1.ResourceName) string {
	if val, exists := resourceList[resource]; exists {
		return val.String()
	}
	return "N/A"
}

func calculateUsedResources(capacity, allocatable corev1.ResourceList) ResourceInfo {
	used := ResourceInfo{}
	used.CPU = calculateResourceUsage(
		getResourceValue(capacity, corev1.ResourceCPU),
		getResourceValue(allocatable, corev1.ResourceCPU),
	)
	used.Memory = calculateResourceUsage(
		getResourceValue(capacity, corev1.ResourceMemory),
		getResourceValue(allocatable, corev1.ResourceMemory),
	)
	used.EphemeralStorage = calculateResourceUsage(
		getResourceValue(capacity, corev1.ResourceEphemeralStorage),
		getResourceValue(allocatable, corev1.ResourceEphemeralStorage),
	)
	used.Storage = calculateResourceUsage(
		getResourceValue(capacity, corev1.ResourceStorage),
		getResourceValue(allocatable, corev1.ResourceStorage),
	)
	return used
}

func calculateFreeResources(capacity, allocatable corev1.ResourceList) ResourceInfo {
	free := ResourceInfo{}
	free.CPU = calculateResourceFree(
		getResourceValue(capacity, corev1.ResourceCPU),
		getResourceValue(allocatable, corev1.ResourceCPU),
	)
	free.Memory = calculateResourceFree(
		getResourceValue(capacity, corev1.ResourceMemory),
		getResourceValue(allocatable, corev1.ResourceMemory),
	)
	free.EphemeralStorage = calculateResourceFree(
		getResourceValue(capacity, corev1.ResourceEphemeralStorage),
		getResourceValue(allocatable, corev1.ResourceEphemeralStorage),
	)
	free.Storage = calculateResourceFree(
		getResourceValue(capacity, corev1.ResourceStorage),
		getResourceValue(allocatable, corev1.ResourceStorage),
	)
	return free
}

func calculateResourceUsage(capacity, allocatable string) string {
	capQty := parseQuantity(capacity)
	allocQty := parseQuantity(allocatable)
	used := capQty.DeepCopy()
	used.Sub(allocQty)
	return used.String()
}

func calculateResourceFree(capacity, allocatable string) string {
	quantity := parseQuantity(allocatable)
	return (&quantity).String()
}

func parseQuantity(value string) resource.Quantity {
	if value == "N/A" {
		return resource.MustParse("0")
	}
	return resource.MustParse(value)
}

func calculateStorageMetrics(capacity resource.Quantity, used string) StorageMetrics {
	usedQty := parseQuantity(used)
	free := capacity.DeepCopy()
	free.Sub(usedQty)
	usagePercent := float64(usedQty.Value()) / float64(capacity.Value()) * 100

	return StorageMetrics{
		TotalCapacity:   capacity,
		UsedCapacity:    usedQty,
		FreeCapacity:    free,
		UsagePercentage: usagePercent,
	}
}
