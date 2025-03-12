// Copyright 2023 bytetrade
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package appdata

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"os"
	"strings"
	"time"
)

var (
	Namespace = os.Getenv("NAMESPACE")
)

func GetAnnotation(ctx context.Context, client *kubernetes.Clientset, key string, bflName string) (string, error) {
	if bflName == "" {
		klog.Error("get Annotation error, bfl-name is empty")
		return "", fmt.Errorf("bfl-name is emtpty")
	}

	namespace := "user-space-" + bflName

	bfl, err := client.AppsV1().StatefulSets(namespace).Get(ctx, "bfl", metav1.GetOptions{})
	if err != nil {
		klog.Errorln("find user's bfl error, ", err, ", ", Namespace)
		return "", err
	}

	klog.Infof("bfl.Annotations: %+v", bfl.Annotations)
	klog.Infof("bfl.Annotations[%s]: %s at time %s", key, bfl.Annotations[key], time.Now().Format(time.RFC3339))
	return bfl.Annotations[key], nil
}

func getPodIP(client *kubernetes.Clientset, prefix, serviceName, namespace, nodeName string) (string, error) {
	pods, err := client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("error listing pods: %w", err)
	}

	var podIP string
	for _, pod := range pods.Items {
		if strings.HasPrefix(pod.GetName(), prefix) && pod.Spec.NodeName == nodeName {
			podIP = pod.Status.PodIP
			break
		}
	}

	if podIP == "" {
		return "", fmt.Errorf("no pod found with the specified prefix and node name")
	}

	return podIP, nil
}

func GetAppDataServiceEndpoint(client *kubernetes.Clientset, nodeName string) string {
	prefix := "appdata-backend"
	serviceName := "appdata-backend-headless"
	namespace := "os-system"

	dnsName, err := getPodIP(client, prefix, serviceName, namespace, nodeName)
	if err != nil {
		klog.Errorf("Error getting Pod DNS name: %s\n", err)
		return ""
	}

	dnsName += ":8110"
	klog.Infoln("Pod DNS name: ", dnsName)
	return dnsName
}
