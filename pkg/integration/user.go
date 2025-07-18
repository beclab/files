package integration

import (
	"context"
	"encoding/json"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (i *integration) getUsers() ([]*User, error) {
	client, err := i.factory.DynamicClient()
	if err != nil {
		return nil, err
	}

	var users []*User

	unstructuredUsers, err := client.Resource(UserGVR).List(context.Background(), v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, unstructuredUser := range unstructuredUsers.Items {
		b, err := unstructuredUser.MarshalJSON()
		if err != nil {
			klog.Errorf("marshal user error: %v", err)
			continue
		}
		var user *User
		if err := json.Unmarshal(b, &user); err != nil {
			klog.Errorf("unmarshal user error: %v", err)
			continue
		}

		users = append(users, user)
	}

	return users, nil

}
