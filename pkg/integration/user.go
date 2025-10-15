package integration

import (
	"files/pkg/client"
	"files/pkg/models"
)

func (i *integration) getUsers() ([]*models.User, error) {
	return client.GetUser(i.client)
}
