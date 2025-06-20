package appdata

import (
	"context"
	"files/pkg/constant"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

var AppData *appData

type appData struct {
	k8sClient    *kubernetes.Clientset
	userPvcMap   map[string]string
	userPvcTime  map[string]time.Time
	cachePvcMap  map[string]string
	cachePvcTime map[string]time.Time
	mu           sync.RWMutex
}

func NewAppData(config *rest.Config) {
	AppData = &appData{
		k8sClient:    kubernetes.NewForConfigOrDie(config),
		userPvcMap:   make(map[string]string),
		userPvcTime:  make(map[string]time.Time),
		cachePvcMap:  make(map[string]string),
		cachePvcTime: make(map[string]time.Time),
	}
}

func (p *appData) GetDriveRootPath() string {
	return constant.ROOT_PREFIX
}

func (p *appData) GetCacheRootPath() string {
	return constant.CACHE_PREFIX
}

func (p *appData) GetExternalRootPath() string {
	return constant.EXTERNAL_PREFIX
}

func (p *appData) GetNodes() (*v1.NodeList, error) {
	var ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	nodes, err := p.k8sClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Errorln("list nodes error, ", err)
		return nil, err
	}

	return nodes, nil
}

func (p *appData) getFromCache(cached func() *string, fetch func() (string, error)) (string, error) {
	if c := func() *string {
		p.mu.RLock()
		defer p.mu.RUnlock()

		return cached()
	}(); c != nil {
		return *c, nil
	}

	return func() (string, error) {
		p.mu.Lock()
		defer p.mu.Unlock()

		if c := cached(); c != nil {
			return *c, nil
		}

		return fetch()
	}()
}

func (p *appData) GetUserPVCOrCache(bflName string) (string, error) {

	return p.getFromCache(
		func() *string {
			if val, ok := p.userPvcMap[bflName]; ok {
				if t, ok := p.userPvcTime[bflName]; ok && time.Since(t) <= 2*time.Minute {
					// p.userPvcTime[bflName] = time.Now()
					return &val
				}
			}

			return nil
		},
		func() (string, error) {
			var ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			userPvc, err := GetAnnotation(ctx, p.k8sClient, "userspace_pvc", bflName)
			if err != nil {
				return "", err
			}

			p.userPvcMap[bflName] = userPvc
			p.userPvcTime[bflName] = time.Now()
			return userPvc, nil
		},
	)

}

func (p *appData) GetCachePVCOrCache(bflName string) (string, error) {
	return p.getFromCache(
		func() *string {
			if val, ok := p.cachePvcMap[bflName]; ok {
				if t, ok := p.cachePvcTime[bflName]; ok && time.Since(t) <= 2*time.Minute {
					// p.cachePvcTime[bflName] = time.Now()
					return &val
				}
			}

			return nil
		},

		func() (string, error) {
			var ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cachePvc, err := GetAnnotation(ctx, p.k8sClient, "appcache_pvc", bflName)
			if err != nil {
				return "", err
			}
			p.cachePvcMap[bflName] = cachePvc
			p.cachePvcTime[bflName] = time.Now()
			return cachePvc, nil
		},
	)
}
