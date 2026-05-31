// Package access provides a single entrypoint for resolving whether a
// user may read or write a resource identified by a frontend URL (or a
// backend physical path). It normalizes the input, validates the user,
// maps the URL to a storage backend, and delegates to that backend's
// unified CheckPermission.
package access

import (
	"context"
	"fmt"

	"files/pkg/drivers"
	"files/pkg/drivers/base"
	"files/pkg/global"
	"files/pkg/integration"
	"files/pkg/models"
)

// CheckAccess resolves the permission Level that owner has on rawURL.
// rawURL is a frontend URL (/{fileType}/{extend}/...). The resolved Level
// is action-independent; the caller decides whether to proceed using
// level.Allow(action).
//
// Share URLs are not handled here: share permission needs the resolved
// share record and member for downstream proxy rewrites and is decided
// by the share middleware via the exported Share* helpers in this
// package. CheckAccess is the entrypoint for storage-backed resources
// (drive/cache/external/cloud/sync); it is wired into the paste handler
// as the src-read / dst-write authorization gate.
func CheckAccess(ctx context.Context, owner, rawURL string) (models.Level, error) {
	if owner == "" {
		return models.LevelNone, fmt.Errorf("owner is required")
	}

	fp, err := models.CreateFileParam(owner, rawURL)
	if err != nil {
		return models.LevelNone, err
	}
	return checkResolved(ctx, owner, fp)
}

// CheckAccessParam is the entrypoint for callers that already hold a
// parsed FileParam (e.g. the resources / upload / archive handlers),
// skipping the URL re-resolution that CheckAccess does. Semantics are
// otherwise identical.
func CheckAccessParam(ctx context.Context, owner string, fp *models.FileParam) (models.Level, error) {
	if owner == "" {
		return models.LevelNone, fmt.Errorf("owner is required")
	}
	if fp == nil {
		return models.LevelNone, fmt.Errorf("file param is required")
	}
	return checkResolved(ctx, owner, fp)
}

// checkResolved validates the owner and dispatches to the storage
// backend's CheckPermission for an already-resolved FileParam.
func checkResolved(ctx context.Context, owner string, fp *models.FileParam) (models.Level, error) {
	if err := validateOwner(owner); err != nil {
		return models.LevelNone, err
	}

	if drivers.Adaptor == nil {
		return models.LevelNone, fmt.Errorf("driver adaptor not initialized")
	}
	handler := drivers.Adaptor.NewFileHandler(fp.FileType, &base.HandlerParam{Ctx: ctx, Owner: owner})
	if handler == nil {
		return models.LevelNone, fmt.Errorf("no handler for file type: %s", fp.FileType)
	}
	return handler.CheckPermission(fp, owner)
}

// validateOwner ensures the owner is a real platform user. A user is
// considered valid if it has a provisioned PVC or appears in the
// integration user list.
func validateOwner(owner string) error {
	if global.GlobalData != nil && global.GlobalData.GetPvcUser(owner) != "" {
		return nil
	}
	if integration.IntegrationService != nil && integration.IntegrationService.UserExists(owner) {
		return nil
	}
	return fmt.Errorf("user not found: %s", owner)
}
