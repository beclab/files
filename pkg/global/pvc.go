package global

import (
	"context"
	"fmt"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
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

	if err := GlobalData.getGlobalData(); err != nil {
		return err
	}

	go func() {
		ticker := time.NewTicker(120 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			GlobalData.getGlobalData()
		}
	}()

	return nil
}

func (g *Data) GetPvcUser(user string) string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.UserPvcMap[user]
}

func (g *Data) GetPvcUserName(pvcUser string) (string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var user string
	for k, v := range g.UserPvcMap {
		if v == pvcUser {
			user = k
			break
		}
	}

	if user == "" {
		return "", fmt.Errorf("userspace pvc not found, name: %s", pvcUser)
	}

	return user, nil
}

func (g *Data) GetPvcCacheName(pvcCache string) (string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var user string
	for k, v := range g.CachePvcMap {
		if v == pvcCache {
			user = k
			break
		}
	}

	if user == "" {
		return "", fmt.Errorf("appcache pvc not found, name: %s", pvcCache)
	}

	return user, nil
}

func (g *Data) GetPvcCache(user string) string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.CachePvcMap[user]
}

func (g *Data) GetPvcCaches() map[string]string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.CachePvcMap
}

func (g *Data) getGlobalData() error {
	users := g.getUsers()
	if users == nil || len(users) == 0 {
		return fmt.Errorf("user not exists")
	}

	for _, user := range users {
		g.getPvc(user)
	}
	return nil
}

func (g *Data) GetGlobalUsers() []string {
	return g.getUsers()
}

func (g *Data) getPvc(user string) {
	var ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespace := "user-space-" + user
	bfl, err := g.k8sClient.AppsV1().StatefulSets(namespace).Get(ctx, "bfl", metav1.GetOptions{})
	if err != nil {
		return
	}

	pvcUser := bfl.Annotations["userspace_pvc"]
	pvcCache := bfl.Annotations["appcache_pvc"]

	GlobalData.mu.Lock()
	GlobalData.UserPvcMap[user] = pvcUser
	GlobalData.CachePvcMap[user] = pvcCache
	GlobalData.mu.Unlock()
}

func (g *Data) getUsers() (users []string) {
	var ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	unstructuredUsers, err := g.k8sInterface.Resource(UsersGVR).List(ctx, metav1.ListOptions{})
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

func (g *Data) HandlerEvent() cache.ResourceEventHandler {
	return cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			var sts = obj.(*appsv1.StatefulSet)
			if sts.Name != "bfl" {
				return false
			}
			return true
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				GlobalData.getGlobalData()
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				GlobalData.getGlobalData()
			},
			DeleteFunc: func(obj interface{}) {
				GlobalData.getGlobalData()
			},
		},
	}
}
