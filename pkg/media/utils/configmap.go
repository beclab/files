package utils

import (
	"context"
	"fmt"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/klog/v2"
)

// GetServiceAccountNamespace reads the namespace from the service account namespace file
func GetServiceAccountNamespace() (string, error) {
	const namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	data, err := os.ReadFile(namespaceFile)
	if err != nil {
		return "", fmt.Errorf("failed to read namespace file: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func GetKubernetesClient() (*kubernetes.Clientset, error) {
	// Use controller-runtime to get Kubernetes configuration
	config := ctrl.GetConfigOrDie()
	klog.Infoln("Using controller-runtime configuration")

	// Create clientset from the configuration
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %v", err)
	}
	return clientset, nil
}

// ReadConfigMap retrieves a ConfigMap from the specified namespace and name.
func ReadConfigMap(clientset *kubernetes.Clientset, namespace, name string) (*corev1.ConfigMap, error) {
	ctx := context.Background()
	configMap, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("ConfigMap %s not found in namespace %s", name, namespace)
		} else if apierrors.IsUnauthorized(err) {
			return nil, fmt.Errorf("unauthorized: check credentials or service account token")
		} else if apierrors.IsForbidden(err) {
			return nil, fmt.Errorf("forbidden: lacks permissions to read ConfigMap %s in namespace %s", name, namespace)
		}
		return nil, fmt.Errorf("failed to read ConfigMap %s: %v", name, err)
	}
	return configMap, nil
}

func WriteConfigMap(clientset *kubernetes.Clientset, namespace, name string, data map[string]string) error {
	// Validate inputs
	if name == "" || namespace == "" {
		return fmt.Errorf("name and namespace must not be empty")
	}
	if len(data) == 0 {
		return fmt.Errorf("data map must not be empty")
	}

	ctx := context.Background()
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: make(map[string]string), // Initialize Data to avoid nil map
	}

	// Try to get existing ConfigMap
	existing, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to check existing ConfigMap %s in namespace %s: %v", name, namespace, err)
	}

	if apierrors.IsNotFound(err) {
		// Create new ConfigMap
		configMap.Data = data // Set input data for creation
		created, err := clientset.CoreV1().ConfigMaps(namespace).Create(ctx, configMap, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create ConfigMap %s in namespace %s: %v", name, namespace, err)
		}
		klog.Infof("Created ConfigMap %s in namespace %s (ResourceVersion: %s)\n", name, namespace, created.ResourceVersion)
		return nil
	}

	// Update existing ConfigMap with append logic
	configMap.ResourceVersion = existing.ResourceVersion // Set ResourceVersion for optimistic locking
	configMap.Data = existing.Data                       // Start with existing Data
	if configMap.Data == nil {
		configMap.Data = make(map[string]string) // Initialize if nil
	}
	// Append input data (merge, with input data taking precedence)
	for key, value := range data {
		configMap.Data[key] = value
	}
	updated, err := clientset.CoreV1().ConfigMaps(namespace).Update(ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ConfigMap %s in namespace %s: %v", name, namespace, err)
	}
	klog.Infof("Updated ConfigMap %s in namespace %s (ResourceVersion: %s)\n", name, namespace, updated.ResourceVersion)

	return nil
}
