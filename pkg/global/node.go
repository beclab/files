package global

import (
	"context"
	"os"
	"sync"
	"time"

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

	// Background-refresh lifecycle. cancel ends the periodic
	// getGlobalNodes loop; <-done blocks until the goroutine has
	// fully exited so a graceful-shutdown coordinator can wait on
	// it. Both are nil until InitGlobalNodes runs.
	cancel context.CancelFunc
	done   chan struct{}
}

func InitGlobalNodes(config *rest.Config) error {
	CurrentNodeName = os.Getenv("NODE_NAME")

	ctx, cancel := context.WithCancel(context.Background())
	GlobalNode = &Node{
		k8sClient: kubernetes.NewForConfigOrDie(config),
		Nodes:     make(map[string]*v1.Node),
		cancel:    cancel,
		done:      make(chan struct{}),
	}

	if err := GlobalNode.getGlobalNodes(); err != nil {
		cancel()
		close(GlobalNode.done)
		return err
	}

	go func() {
		defer close(GlobalNode.done)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := GlobalNode.getGlobalNodes(); err != nil {
					klog.Warningf("[global] tick refresh of cluster nodes failed: %v", err)
				}
			}
		}
	}()

	return nil
}

// Stop ends the background refresh goroutine and waits for it to
// exit. Safe to call multiple times.
func (g *Node) Stop(ctx context.Context) error {
	if g == nil || g.cancel == nil {
		return nil
	}
	g.cancel()
	select {
	case <-g.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
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
				if err := GlobalNode.getGlobalNodes(); err != nil {
					klog.Warningf("[global] node Add refresh failed: %v", err)
				}
			},
			DeleteFunc: func(obj interface{}) {
				if err := GlobalNode.getGlobalNodes(); err != nil {
					klog.Warningf("[global] node Delete refresh failed: %v", err)
				}
			},
		},
	}
}
