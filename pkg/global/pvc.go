package global

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	GlobalData *Data

	UsersGVR = schema.GroupVersionResource{
		Group:    "iam.kubesphere.io",
		Version:  "v1alpha2",
		Resource: "users",
	}
)

type Data struct {
	k8sClient    *kubernetes.Clientset
	k8sInterface dynamic.Interface
	UserPvcMap   map[string]string
	CachePvcMap  map[string]string
	mu           sync.RWMutex
}

func InitGlobalData(config *rest.Config) error {
	GlobalData = &Data{
		k8sClient:    kubernetes.NewForConfigOrDie(config),
		k8sInterface: dynamic.NewForConfigOrDie(config),
		UserPvcMap:   make(map[string]string),
		CachePvcMap:  make(map[string]string),
	}

	if err := GlobalData.getGlobalData(GlobalData.k8sInterface, GlobalData.k8sClient); err != nil {
		return err
	}

	go func() {
		ticker := time.NewTicker(120 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			GlobalData.getGlobalData(GlobalData.k8sInterface, GlobalData.k8sClient)
		}
	}()

	return nil
}

func (g *Data) GetPvcUser(user string) string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.UserPvcMap[user]
}

func (g *Data) GetPvcCache(user string) string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.CachePvcMap[user]
}

func (g *Data) getGlobalData(k8sInterface dynamic.Interface, k8sClient *kubernetes.Clientset) error {
	users := g.getUsers(k8sInterface)
	if users == nil || len(users) == 0 {
		return fmt.Errorf("user not exists")
	}

	for _, user := range users {
		g.getPvc(user, k8sClient)
	}
	return nil
}

func (g *Data) getPvc(user string, client *kubernetes.Clientset) {
	var ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespace := "user-space-" + user
	bfl, err := client.AppsV1().StatefulSets(namespace).Get(ctx, "bfl", metav1.GetOptions{})
	if err != nil {
		return
	}

	pvcUser := bfl.Annotations["userspace_pvc"]
	pvcCache := bfl.Annotations["appcache_pvc"]

	GlobalData.mu.Lock()
	GlobalData.UserPvcMap[user] = pvcUser
	GlobalData.CachePvcMap[user] = pvcCache
	GlobalData.mu.Unlock()

	return
}

func (g *Data) getUsers(k8sInterface dynamic.Interface) (users []string) {
	var ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	unstructuredUsers, err := k8sInterface.Resource(UsersGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}
	for _, unstructuredUser := range unstructuredUsers.Items {
		obj := unstructuredUser.UnstructuredContent()
		olaresId, _, err := unstructured.NestedString(obj, "metadata", "name")
		if err != nil {
			continue
		}
		if olaresId == "" {
			continue
		}
		users = append(users, olaresId)
	}

	return
}
