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
	"errors"
	"os"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	UserAppDataDeployName    = "appdata-backend"
	UserAppDataDeployService = "appdata-backend"
)

var (
	Namespace        = ""
	OS_SYSTEM_SERVER = ""
	FileImage        = ""

	oneReplica int32 = 1

	// require:
	//   name with node
	//   namespace
	//   node affinity
	//   volumes
	//   OS_SYSTEM_SERVER
	AppDataDeploy = appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: UserAppDataDeployName,
			Labels: map[string]string{
				"app": UserAppDataDeployName,
			},
			Annotations: map[string]string{
				"velero.io/exclude-from-backup": "true",
			},
		},

		Spec: appsv1.DeploymentSpec{
			Replicas: &oneReplica,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": UserAppDataDeployName,
				},
			},

			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": UserAppDataDeployName,
					},
				},

				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "files",
							Image:           os.Getenv("FILES_SERVER_TAG"), //"beclab/files-server:v0.2.24",
							ImagePullPolicy: corev1.PullIfNotPresent,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "fb-data",
									MountPath: "/appdata",
								},
								{
									Name:      "user-appdata-dir",
									MountPath: "/data/AppData",
								},
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8110,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "FB_DATABASE",
									Value: "/appdata/database/filebrowser.db",
								},
								{
									Name:  "FB_CONFIG",
									Value: "/appdata/config/settings.json",
								},
								{
									Name:  "FB_ROOT",
									Value: "/data",
								},
								{
									Name:  "OS_SYSTEM_SERVER",
									Value: "",
								},
							},
							Command: []string{
								"/filebrowser",
								"--noauth",
							},
						},
					}, // end of containers

					// volumes:
					// - name: user-appdata-dir
					//   persistentVolumeClaim:
					// 	claimName: {{ .Values.pvc.userspace }}
					// - name: fb-data
					//   hostPath:
					// 	type: DirectoryOrCreate
					// 	path: {{ .Values.userspace.appdata}}/files

					Volumes: []corev1.Volume{},
				},
			},
		},
	} // end of AppDataDeploy template

	AppDataDeployService = corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: UserAppDataDeployService,
		},

		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": UserAppDataDeployName,
			},
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "appdata",
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromInt(8110),
				},
			},
		},
	}
)

func init() {
	Namespace = os.Getenv("NAMESPACE")
	OS_SYSTEM_SERVER = os.Getenv("OS_SYSTEM_SERVER")
	FileImage = os.Getenv("FILE_IMAGE")
}

func GetAnnotation(ctx context.Context, client *kubernetes.Clientset, nodeName string, key string, bflName string) (string, error) {
	if bflName == "" {
		klog.Error("get Annotation error, bfl-name is empty")
		return "", errors.New("bfl-name is emtpty")
	}

	namespace := "user-space-" + bflName

	bfl, err := client.AppsV1().StatefulSets(namespace).Get(ctx, "bfl", metav1.GetOptions{})
	if err != nil {
		klog.Error("find user's bfl error, ", err, ", ", Namespace)
		return "", err
	}

	klog.Infof("bfl.Annotations: %+v", bfl.Annotations)
	return bfl.Annotations[key], nil
}

func GetAppDataDeploymentDef(ctx context.Context, client *kubernetes.Clientset, nodeName string) (*appsv1.Deployment, error) {
	if Namespace == "" {
		klog.Error("get appdata deployment error, namespace is empty")
		return nil, errors.New("namespace is emtpty")
	}

	if OS_SYSTEM_SERVER == "" {
		// try to find OS_SYSTEM_SERVER value
		OS_SYSTEM_SERVER = "system-server." + strings.Replace(Namespace, "user-space-", "user-system-", 1)
	}

	bfl, err := client.AppsV1().StatefulSets(Namespace).Get(ctx, "bfl", metav1.GetOptions{})
	if err != nil {
		klog.Error("find user's bfl error, ", err, ", ", Namespace)
		return nil, err
	}

	// find user's appdata hostpath
	appdata_hostpath, ok := bfl.Annotations["appcache_hostpath"]
	if !ok {
		err = errors.New("appdata host path not found")
		klog.Error("find user's bfl error, ", err, ", ", Namespace)

		return nil, err
	}

	deployment := AppDataDeploy.DeepCopy()
	deployment.Namespace = Namespace
	deployment.Name = deployment.Name + "-" + nodeName
	deployment.Spec.Selector.MatchLabels["app"] = getMatchAppLabel(nodeName)
	deployment.Spec.Template.ObjectMeta.Labels["app"] = getMatchAppLabel(nodeName)

	volumeType := corev1.HostPathDirectory
	volumeTypeCreate := corev1.HostPathDirectoryOrCreate
	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, []corev1.Volume{
		{
			Name: "user-appdata-dir",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Type: &volumeType,
					Path: appdata_hostpath,
				},
			},
		},
		{
			Name: "fb-data",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Type: &volumeTypeCreate,
					Path: appdata_hostpath + "/files-appdata",
				},
			},
		},
	}...)

	deployment.Spec.Template.Spec.Affinity = &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "kubernetes.io/hostname",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{nodeName},
							},
						},
					},
				},
			},
		},
	}

	for i, c := range deployment.Spec.Template.Spec.Containers {
		if FileImage != "" && c.Name == "files" {
			c.Image = FileImage
		}

		for n, e := range c.Env {
			if e.Name == "OS_SYSTEM_SERVER" {
				deployment.Spec.Template.Spec.Containers[i].Env[n].Value = OS_SYSTEM_SERVER
				break
			}
		}
	}

	return deployment, nil
}

func GetAppDataServiceDef(nodeName string) (*corev1.Service, error) {
	if Namespace == "" {
		klog.Error("get appdata service error, namespace is empty")
		return nil, errors.New("namespace is emtpty")
	}

	service := AppDataDeployService.DeepCopy()
	service.Name = service.Name + "-" + nodeName
	service.Namespace = Namespace
	service.Spec.Selector["app"] = getMatchAppLabel(nodeName)

	return service, nil
}

func getMatchAppLabel(nodeName string) string {
	return UserAppDataDeployName + "-" + nodeName
}

func GetAppDataServiceEndpoint(nodeName string) string {
	return UserAppDataDeployService + "-" + nodeName + "." + Namespace
}
