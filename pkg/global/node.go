package global

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
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

func (g *Node) CheckDiskPressure() (bool, error) {
	node, exists := g.Nodes[CurrentNodeName]
	if !exists {
		klog.Infof("Get node info failed")
		return false, fmt.Errorf("get node info failed") // TODO
	}

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
