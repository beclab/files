package rpc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sync"
)

var PVCs *PVCCache = nil

type PVCCache struct {
	service     *Service
	userPvcMap  map[string]string
	cachePvcMap map[string]string
	mu          sync.Mutex
}

func NewPVCCache(service *Service) *PVCCache {
	return &PVCCache{
		service:     service,
		userPvcMap:  make(map[string]string),
		cachePvcMap: make(map[string]string),
	}
}

func (p *PVCCache) getBflForUserPVCOrCache(userPvc string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if val, ok := p.userPvcMap[userPvc]; ok {
		return val, nil
	}

	_, bflName, err := FindStatefulSetByPVCAnnotation(p.service.context, p.service.k8sClient, "userspace_pvc", userPvc)
	if err != nil {
		return "", err
	}
	p.userPvcMap[userPvc] = bflName
	return bflName, nil
}

func (p *PVCCache) getBflForCachePVCOrCache(cachePvc string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if val, ok := p.cachePvcMap[cachePvc]; ok {
		return val, nil
	}

	_, bflName, err := FindStatefulSetByPVCAnnotation(p.service.context, p.service.k8sClient, "appcache_pvc", cachePvc)
	if err != nil {
		return "", err
	}
	p.cachePvcMap[cachePvc] = bflName
	return bflName, nil
}

func (p *PVCCache) getBfl(pvc string) (string, error) {
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
		klog.Error("Failed to list namespaces: ", err)
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
				fmt.Println("userspace: ", ns.Name, "bfl_name: ", ns.Name[len("user-space-"):])
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
	} else if strings.HasPrefix(path, CacheRootPath) {
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
		fmt.Println("Error walking the path:", err)
	}
	fmt.Println("Oringinal Paths: ", A)
	fmt.Println("Expanded Paths: ", B)
	return B
}
