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

package operator

import (
	"errors"

	"github.com/Above-Os/files/gateway/pkg/appdata"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog/v2"
)

func (w *Watcher) reconcile(event *watch.Event) error {
	node, ok := event.Object.(*corev1.Node)
	if !ok {
		klog.Error("invalid object to watch, ", event.Object.GetObjectKind())
		return errors.New("invalid object")
	}

	klog.Info("fire watched event, ", event.Type, ", ", node.Name, ", ", node.Namespace)

	switch event.Type {
	case watch.Deleted:
		if err := w.deleteDeploy(node); err != nil {
			return err
		}

	case watch.Added:
		if err := w.createDeploy(node); err != nil {
			return err
		}

	default:
		klog.Info("ignore event, ", event.Type)

	}
	return nil
}

func (w *Watcher) deleteDeploy(node *corev1.Node) error {
	deploy, err := appdata.GetAppDataDeploymentDef(w.ctx, w.client, node.Name)
	if err != nil {
		return err
	}

	klog.Info("delete appdata from node, ", deploy.Name, ", ", deploy.Namespace)
	err = w.client.AppsV1().Deployments(deploy.Namespace).Delete(w.ctx, deploy.Name, metav1.DeleteOptions{})

	if err != nil && !apierrors.IsNotFound(err) {
		klog.Error("delete appdata deploy on node error, ", err, ", ", deploy.Name, ", ", deploy.Namespace)
		return err
	}

	service, err := appdata.GetAppDataServiceDef(node.Name)
	if err != nil {
		return err
	}

	klog.Info("delete appdata service from node, ", service.Name, ", ", service.Namespace)
	err = w.client.CoreV1().Services(service.Namespace).Delete(w.ctx, service.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		klog.Error("delete appdata service on node error, ", err, ", ", service.Name, ", ", service.Namespace)
		return err
	}

	return nil
}

func (w *Watcher) createDeploy(node *corev1.Node) error {
	deploy, err := appdata.GetAppDataDeploymentDef(w.ctx, w.client, node.Name)
	if err != nil {
		return err
	}

	klog.Info("create appdata to node, ", deploy.Name, ", ", deploy.Namespace)
	_, err = w.client.AppsV1().Deployments(deploy.Namespace).Get(w.ctx, deploy.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		klog.Error("find appdata deploy on node error, ", err, ", ", deploy.Name, ", ", deploy.Namespace)
		return err
	}

	if err == nil {
		klog.Info("appdata is already deployed on node, ", deploy.Name, ", ", deploy.Namespace)
	} else {
		_, err = w.client.AppsV1().Deployments(deploy.Namespace).Create(w.ctx, deploy, metav1.CreateOptions{})
		if err != nil {
			klog.Error("create appdata deploy on node error, ", err, ", ", deploy.Name, ", ", deploy.Namespace)
			return err
		}
	}

	service, err := appdata.GetAppDataServiceDef(node.Name)
	if err != nil {
		return err
	}
	klog.Info("create appdata service to node, ", service.Name, ", ", service.Namespace)
	_, err = w.client.CoreV1().Services(service.Namespace).Get(w.ctx, service.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		klog.Error("find appdata service on node error, ", err, ", ", service.Name, ", ", service.Namespace)
		return err
	}

	if err == nil {
		klog.Info("appdata service is already deployed on node, ", service.Name, ", ", service.Namespace)
	} else {
		_, err = w.client.CoreV1().Services(service.Namespace).Create(w.ctx, service, metav1.CreateOptions{})
		if err != nil {
			klog.Error("create appdata service on node error, ", err, ", ", deploy.Name, ", ", deploy.Namespace)
			return err
		}
	}

	return nil
}
