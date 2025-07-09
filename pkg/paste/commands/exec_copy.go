package commands

import (
	"context"
	"files/pkg/models"
)

func (c *command) ExecSyncCopy(ctx context.Context, src *models.FileParam, dst *models.FileParam) error {
	return nil
}

func (c *command) ExecCloudCopy(ctx context.Context, src *models.FileParam, dst *models.FileParam) error {
	// run rclone
	return nil
}
