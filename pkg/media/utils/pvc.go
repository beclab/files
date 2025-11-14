package utils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	BFL_HEADER = "X-Bfl-User"
)

type PVCCache struct {
	mainCtx     context.Context
	k8sClient   *kubernetes.Clientset
	userPvcMap  map[string]string
	cachePvcMap map[string]string
	mu          sync.Mutex
}

func NewPVCCache(ctx context.Context, kubeConfig *rest.Config) *PVCCache {
	return &PVCCache{
		mainCtx:     ctx,
		k8sClient:   kubernetes.NewForConfigOrDie(kubeConfig),
		userPvcMap:  make(map[string]string),
		cachePvcMap: make(map[string]string),
	}
}

func (p *PVCCache) GetUserPVCOrCache(bflName string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if val, ok := p.userPvcMap[bflName]; ok {
		return val, nil
	}

	userPvc, err := GetAnnotation(p.mainCtx, p.k8sClient, "userspace_pvc", bflName)
	if err != nil {
		return "", err
	}
	p.userPvcMap[bflName] = userPvc
	return userPvc, nil
}

func (p *PVCCache) GetCachePVCOrCache(bflName string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if val, ok := p.cachePvcMap[bflName]; ok {
		return val, nil
	}

	cachePvc, err := GetAnnotation(p.mainCtx, p.k8sClient, "appcache_pvc", bflName)
	if err != nil {
		return "", err
	}
	p.cachePvcMap[bflName] = cachePvc
	return cachePvc, nil
}

func GetAnnotation(ctx context.Context, client *kubernetes.Clientset, key string, bflName string) (string, error) {
	if bflName == "" {
		klog.Error("get Annotation error, bfl-name is empty")
		return "", errors.New("bfl-name is emtpty")
	}

	namespace := "user-space-" + bflName

	bfl, err := client.AppsV1().StatefulSets(namespace).Get(ctx, "bfl", metav1.GetOptions{})
	if err != nil {
		klog.Error("find user's bfl error, ", err, ", ", namespace)
		return "", err
	}

	klog.Infof("bfl.Annotations: %+v", bfl.Annotations)
	return bfl.Annotations[key], nil
}

var PvcCache *PVCCache

func InitPVC(ctx context.Context) {
	config := ctrl.GetConfigOrDie()
	fmt.Printf("%+v\n", config)

	PvcCache = NewPVCCache(ctx, config)
}

func GetEffectPlayPath(playPath, bflName string) (string, error) {
	var effectPlayPath string
	prefix := "/AppData"
	if strings.HasPrefix(playPath, prefix) {
		cachePVC, err := PvcCache.GetCachePVCOrCache(bflName)
		if err != nil {
			klog.Info(err)
			return "", errors.New("get cache pvc name error")
		} else {
			klog.Info("appcache pvc: ", cachePVC)
		}

		cacheDir := os.Getenv("MEDIA_SERVER_CACHE_DIR")
		effectPlayPath = cacheDir + "/" + cachePVC + playPath[len(prefix):]
	} else {
		userPVC, err := PvcCache.GetUserPVCOrCache(bflName)
		if err != nil {
			klog.Info(err)
			return "", errors.New("get user pvc name error")
		} else {
			klog.Info("user-space pvc: ", userPVC)
		}

		dataDir := os.Getenv("MEDIA_SERVER_DATA_DIR")
		prefix := "/Application"
		if strings.HasPrefix(playPath, prefix) {
			effectPlayPath = dataDir + "/" + userPVC + "/Data" + playPath[len(prefix):]
		} else {
			effectPlayPath = dataDir + "/" + userPVC + playPath
		}
	}

	return effectPlayPath, nil
}
