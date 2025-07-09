package commands

import (
	"context"
	"files/pkg/models"
)

func (c *command) ExecUploadToSync(ctx context.Context, src *models.FileParam, dst *models.FileParam) error {
	return nil
}

func (c *command) ExecUploadToCloud(ctx context.Context, src *models.FileParam, dst *models.FileParam) error {
	// run rclone
	return nil
}
