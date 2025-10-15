package client

import (
	"context"
	"encoding/json"
	"files/pkg/models"

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
