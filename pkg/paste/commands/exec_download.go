package commands

import (
	"context"
	"files/pkg/models"
)

func (c *command) ExecDownloadFromFiles(ctx context.Context, src *models.FileParam, dst *models.FileParam) error {
	// run rsync
	return nil
}

func (c *command) ExecDownloadFromSync(ctx context.Context, src *models.FileParam, dst *models.FileParam) error {
	return nil
}

func (c *command) ExecDownloadFromCloud(ctx context.Context, src *models.FileParam, dst *models.FileParam) error {
	// run rclone
	return nil
}
