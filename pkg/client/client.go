package client

import (
	"context"
	"encoding/json"
	"files/pkg/models"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

func GetUser(client *dynamic.DynamicClient) ([]*models.User, error) {
	var users []*models.User

	unstructuredUsers, err := client.Resource(models.UserGVR).List(context.Background(), v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, unstructuredUser := range unstructuredUsers.Items {
		b, err := unstructuredUser.MarshalJSON()
		if err != nil {
			klog.Errorf("marshal user error: %v", err)
			continue
		}
		var user *models.User
		if err := json.Unmarshal(b, &user); err != nil {
			klog.Errorf("unmarshal user error: %v", err)
			continue
		}

		users = append(users, user)
	}

	return users, nil
}

func DeleteShareRole(user string) error {
	f, err := NewFactory()
	if err != nil {
		return err
	}

	client, err := f.KubeClient()
	if err != nil {
		return err
	}

	var shareClusterRole = fmt.Sprintf("%s:files-frontend-domain-share", user)
	if err = client.RbacV1().ClusterRoles().Delete(context.Background(), shareClusterRole, v1.DeleteOptions{}); err != nil {
		return err

	}

	var shareClusterRoleBinding = fmt.Sprintf("user:%s:files-frontend-domain-share", user)
	if err = client.RbacV1().ClusterRoleBindings().Delete(context.Background(), shareClusterRoleBinding, v1.DeleteOptions{}); err != nil {
		return err
	}

	return nil
}

func CreateShareRole(user string) error {
	f, err := NewFactory()
	if err != nil {
		return err
	}

	client, err := f.KubeClient()
	if err != nil {
		return err
	}

	var shareClusterRole = fmt.Sprintf("%s:files-frontend-domain-share", user)
	_, err = client.RbacV1().ClusterRoles().Get(context.Background(), shareClusterRole, v1.GetOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		var newRole = &rbacv1.ClusterRole{
			ObjectMeta: v1.ObjectMeta{
				Name: shareClusterRole,
				Annotations: map[string]string{
					"provider-registry-ref": fmt.Sprintf("%s/share", user),
					"provider-service-ref":  "files-service.os-framework:80",
				},
			},
			Rules: []rbacv1.PolicyRule{
				rbacv1.PolicyRule{
					NonResourceURLs: []string{"*"},
					Verbs:           []string{"*"},
				},
			},
		}
		_, err = client.RbacV1().ClusterRoles().Create(context.Background(), newRole, v1.CreateOptions{})
		if err != nil {
			klog.Errorf("create cluster role error: %v", err)
			return err
		}
	} else if err != nil {
		klog.Errorf("get cluster role error: %v", err)
	}

	var shareClusterRoleBinding = fmt.Sprintf("user:%s:files-frontend-domain-share", user)
	_, err = client.RbacV1().ClusterRoleBindings().Get(context.Background(), shareClusterRoleBinding, v1.GetOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		var newRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: v1.ObjectMeta{
				Name: shareClusterRoleBinding,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     fmt.Sprintf("%s:files-frontend-domain-share", user),
			},
			Subjects: []rbacv1.Subject{
				rbacv1.Subject{
					Kind: "User",
					Name: user,
				},
			},
		}
		_, err = client.RbacV1().ClusterRoleBindings().Create(context.Background(), newRoleBinding, v1.CreateOptions{})
		if err != nil {
			klog.Errorf("create cluster role binding error: %v", err)
			return err
		}
	} else if err != nil {
		klog.Errorf("get cluster role binding error: %v", err)
	}

	return nil
}
