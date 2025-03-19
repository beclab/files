package rpc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sync"
)

// PVC->Bfl
var PVCs *PVCCache = nil

type PVCCache struct {
	service      *Service
	userPvcMap   map[string]string
	userPvcTime  map[string]time.Time
	cachePvcMap  map[string]string
	cachePvcTime map[string]time.Time
	mu           sync.Mutex
}

func NewPVCCache(service *Service) *PVCCache {
	return &PVCCache{
		service:      service,
		userPvcMap:   make(map[string]string),
		userPvcTime:  make(map[string]time.Time),
		cachePvcMap:  make(map[string]string),
		cachePvcTime: make(map[string]time.Time),
	}
}

func (p *PVCCache) getBflForUserPVCOrCache(userPvc string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if val, ok := p.userPvcMap[userPvc]; ok {
		if t, ok := p.userPvcTime[userPvc]; ok && time.Since(t) <= 2*time.Minute {
			p.userPvcTime[userPvc] = time.Now()
			return val, nil
		}
	}

	_, bflName, err := FindStatefulSetByPVCAnnotation(p.service.context, p.service.k8sClient, "userspace_pvc", userPvc)
	if err != nil {
		return "", err
	}
	p.userPvcMap[userPvc] = bflName
	p.userPvcTime[userPvc] = time.Now()
	return bflName, nil
}

func (p *PVCCache) getBflForCachePVCOrCache(cachePvc string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if val, ok := p.cachePvcMap[cachePvc]; ok {
		if t, ok := p.cachePvcTime[cachePvc]; ok && time.Since(t) <= 2*time.Minute {
			p.cachePvcTime[cachePvc] = time.Now()
			return val, nil
		}
	}

	_, bflName, err := FindStatefulSetByPVCAnnotation(p.service.context, p.service.k8sClient, "appcache_pvc", cachePvc)
	if err != nil {
		return "", err
	}
	p.cachePvcMap[cachePvc] = bflName
	p.cachePvcTime[cachePvc] = time.Now()
	return bflName, nil
}

func (p *PVCCache) GetBfl(pvc string) (string, error) {
	bflName, err := p.getBflForUserPVCOrCache(pvc)
	if bflName == "" || err != nil {
		bflName, err = p.getBflForCachePVCOrCache(pvc)
		if bflName == "" || err != nil {
			return "", err
		}
	}
	return bflName, nil
}

func FindStatefulSetByPVCAnnotation(ctx context.Context, client *kubernetes.Clientset, key string, pvcIdentifier string) (string, string, error) {
	namespaces, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Errorln("Failed to list namespaces: ", err)
		return "", "", err
	}

	for _, ns := range namespaces.Items {
		statefulSets, err := client.AppsV1().StatefulSets(ns.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			klog.Errorf("Failed to list StatefulSets in namespace %s: %v", ns.Name, err)
			continue
		}

		for _, bfl := range statefulSets.Items {
			if value, ok := bfl.Annotations[key]; ok && value == pvcIdentifier {
				klog.Infoln("userspace: ", ns.Name, "bfl_name: ", ns.Name[len("user-space-"):], "at time: ", time.Now().Format(time.RFC3339))
				return ns.Name, ns.Name[len("user-space-"):], nil
			}
		}
	}

	return "", "", fmt.Errorf("no matching StatefulSet found for PVC identifier %s and key %s", pvcIdentifier, key)
}

func ExtractPvcFromURL(path string) string {
	splitPrefix := ""
	if strings.HasPrefix(path, RootPrefix) {
		splitPrefix = RootPrefix
	} else if strings.HasPrefix(path, CacheRootPath) || strings.HasPrefix(path, AppDataRootPath) {
		splitPrefix = CacheRootPath
	} else {
		return ""
	}

	trimmedPath := strings.TrimPrefix(path, splitPrefix)

	firstSlash := strings.Index(trimmedPath, "/")
	if firstSlash == -1 {
		return ""
	}

	secondSlash := strings.Index(trimmedPath[firstSlash+1:], "/")
	if secondSlash == -1 {
		return trimmedPath[firstSlash+1:]
	}

	return trimmedPath[firstSlash+1 : firstSlash+1+secondSlash]
}

func ExpandPaths(A []string, prefix string) []string {
	var B []string
	err := filepath.Walk(prefix, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		for _, element := range A {
			if strings.HasSuffix(path, element) {
				B = append(B, path)
			}
		}
		return nil
	})
	if err != nil {
		klog.Errorln("Error walking the path:", err)
	}
	klog.Infoln("Oringinal Paths: ", A)
	klog.Infoln("Expanded Paths: ", B)
	return B
}

// Bfl->PVC
var BflPVCs *BflPVCCache = nil

type BflPVCCache struct {
	service      *Service
	userPvcMap   map[string]string
	userPvcTime  map[string]time.Time
	cachePvcMap  map[string]string
	cachePvcTime map[string]time.Time
	mu           sync.Mutex
}

func NewBflPVCCache(service *Service) *BflPVCCache {
	return &BflPVCCache{
		service:      service,
		userPvcMap:   make(map[string]string),
		userPvcTime:  make(map[string]time.Time),
		cachePvcMap:  make(map[string]string),
		cachePvcTime: make(map[string]time.Time),
	}
}

func GetAnnotation(ctx context.Context, client *kubernetes.Clientset, key string, bflName string) (string, error) {
	if bflName == "" {
		klog.Error("get Annotation error, bfl-name is empty")
		return "", errors.New("bfl-name is emtpty")
	}

	namespace := "user-space-" + bflName

	bfl, err := client.AppsV1().StatefulSets(namespace).Get(ctx, "bfl", metav1.GetOptions{})
	if err != nil {
		klog.Errorln("find user's bfl error, ", err, ", ", namespace)
		return "", err
	}

	klog.Infof("bfl.Annotations: %+v", bfl.Annotations)
	klog.Infof("bfl.Annotations[%s]: %s at time %s", key, bfl.Annotations[key], time.Now().Format(time.RFC3339))
	return bfl.Annotations[key], nil
}

func (p *BflPVCCache) getUserPVCOrCache(bflName string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if val, ok := p.userPvcMap[bflName]; ok {
		if t, ok := p.userPvcTime[bflName]; ok && time.Since(t) <= 2*time.Minute {
			p.userPvcTime[bflName] = time.Now()
			return val, nil
		}
	}

	userPvc, err := GetAnnotation(p.service.context, p.service.k8sClient, "userspace_pvc", bflName)
	if err != nil {
		return "", err
	}
	p.userPvcMap[bflName] = userPvc
	p.userPvcTime[bflName] = time.Now()
	return userPvc, nil
}

func (p *BflPVCCache) getCachePVCOrCache(bflName string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if val, ok := p.cachePvcMap[bflName]; ok {
		if t, ok := p.cachePvcTime[bflName]; ok && time.Since(t) <= 2*time.Minute {
			p.cachePvcTime[bflName] = time.Now()
			return val, nil
		}
	}

	cachePvc, err := GetAnnotation(p.service.context, p.service.k8sClient, "appcache_pvc", bflName)
	if err != nil {
		return "", err
	}
	p.cachePvcMap[bflName] = cachePvc
	p.cachePvcTime[bflName] = time.Now()
	return cachePvc, nil
}
