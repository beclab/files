package global

import (
	"context"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	GlobalNode *Node
)

type NodeInfo struct {
	Name string `json:"name"`
}

type Node struct {
	k8sClient *kubernetes.Clientset
	Nodes     map[string]*v1.Node
	mu        sync.RWMutex
}

func InitGlobalNodes(config *rest.Config) error {
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
		var node = NodeInfo{
			Name: n.Name,
		}
		nodes = append(nodes, node)
	}

	return nodes
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
		return err
	}

	for _, node := range nodes.Items {
		g.Nodes[node.Name] = &node
	}

	return nil
}
