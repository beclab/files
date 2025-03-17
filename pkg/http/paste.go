package http

import (
	"context"
	"files/pkg/common"
	"files/pkg/drives"
	"files/pkg/errors"
	"files/pkg/files"
	"files/pkg/fileutils"
	"fmt"
	"github.com/spf13/afero"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"path"
	"strings"
)

func resourcePasteHandler(fileCache fileutils.FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		src := r.URL.Path
		dst := r.URL.Query().Get("destination")
		srcType := r.URL.Query().Get("src_type")
		if srcType == "" {
			srcType = drives.SrcTypeDrive
		}
		dstType := r.URL.Query().Get("dst_type")
		if dstType == "" {
			dstType = drives.SrcTypeDrive
		}

		if !drives.ValidSrcTypes[srcType] {
			klog.Infoln("Src type is invalid!")
			return http.StatusForbidden, nil
		}
		if !drives.ValidSrcTypes[dstType] {
			klog.Infoln("Dst type is invalid!")
			return http.StatusForbidden, nil
		}
		if srcType == dstType {
			klog.Infoln("Src and dst are of same arch!")
		} else {
			klog.Infoln("Src and dst are of different arches!")
		}
		action := r.URL.Query().Get("action")
		var err error
		klog.Infoln("src:", src)
		src, err = common.UnescapeURLIfEscaped(src)
		klog.Infoln("src:", src, "err:", err)
		klog.Infoln("dst:", dst)
		dst, err = common.UnescapeURLIfEscaped(dst)
		klog.Infoln("dst:", dst, "err:", err)
		if err != nil {
			return common.ErrToStatus(err), err
		}
		if dst == "/" || src == "/" {
			return http.StatusForbidden, nil
		}

		if dstType == drives.SrcTypeSync && strings.Contains(dst, "\\") {
			response := map[string]interface{}{
				"code": -1,
				"msg":  "Sync does not support directory entries with backslashes in their names.",
			}
			return common.RenderJSON(w, r, response)
		}

		rename := r.URL.Query().Get("rename") == "true"
		if !rename {
			if _, err := files.DefaultFs.Stat(dst); err == nil {
				return http.StatusConflict, nil
			}
		}
		if srcType == drives.SrcTypeGoogle && dstType != drives.SrcTypeGoogle {
			srcInfo, err := drives.GetGoogleDriveIdFocusedMetaInfos(src, w, r)
			if err != nil {
				return http.StatusInternalServerError, err
			}
			srcName := srcInfo.Name
			formattedSrcName := common.RemoveSlash(srcName)
			dst = strings.ReplaceAll(dst, srcName, formattedSrcName)

			if !srcInfo.CanDownload {
				if srcInfo.CanExport {
					dst += srcInfo.ExportSuffix
				} else {
					response := map[string]interface{}{
						"code": -1,
						"msg":  "Google drive cannot export this file.",
					}
					return common.RenderJSON(w, r, response)
				}
			}
		}
		if rename && dstType != drives.SrcTypeGoogle {
			dst = drives.PasteAddVersionSuffix(dst, dstType, files.DefaultFs, w, r)
		}
		var same = srcType == dstType
		// all cloud drives of two users must be seen as diff archs
		var srcName, dstName string
		if srcType == drives.SrcTypeGoogle {
			_, srcName, _, _ = drives.ParseGoogleDrivePath(src)
		} else if drives.IsCloudDrives(srcType) {
			_, srcName, _ = drives.ParseCloudDrivePath(src, true)
		}
		if dstType == drives.SrcTypeGoogle {
			_, dstName, _, _ = drives.ParseGoogleDrivePath(dst)
		} else if drives.IsCloudDrives(srcType) {
			_, dstName, _ = drives.ParseCloudDrivePath(dst, true)
		}
		if srcName != dstName {
			same = false
		}

		if same {
			err = pasteActionSameArch(action, srcType, src, dstType, dst, rename, fileCache, w, r)
		} else {
			err = pasteActionDiffArch(r.Context(), action, srcType, src, dstType, dst, d, fileCache, w, r)
		}
		if common.ErrToStatus(err) == http.StatusRequestEntityTooLarge {
			fmt.Fprintln(w, err.Error())
		}
		return common.ErrToStatus(err), err
	}
}

func doPaste(fs afero.Fs, srcType, src, dstType, dst string, d *common.Data, w http.ResponseWriter, r *http.Request) error {
	// path.Clean, only operate on string level, so it fits every src/dst type.
	if src = path.Clean("/" + src); src == "" {
		return os.ErrNotExist
	}

	if dst = path.Clean("/" + dst); dst == "" {
		return os.ErrNotExist
	}

	if src == "/" || dst == "/" {
		// Prohibit copying from or to the virtual root directory.
		return os.ErrInvalid
	}

	// Only when URL and type are both the same, it is not OK.
	if (dst == src) && (dstType == srcType) {
		return os.ErrInvalid
	}

	handler, err := drives.GetResourceService(srcType)
	if err != nil {
		return err
	}

	_, size, mode, isDir, err := handler.GetStat(fs, src, w, r)
	if err != nil {
		return err
	}

	var copyTempGoogleDrivePathIdCache = make(map[string]string)

	if isDir {
		err = handler.PasteDirFrom(fs, srcType, src, dstType, dst, d, mode, w, r, copyTempGoogleDrivePathIdCache)
	} else {
		err = handler.PasteFileFrom(fs, srcType, src, dstType, dst, d, mode, size, w, r, copyTempGoogleDrivePathIdCache)
	}
	if err != nil {
		return err
	}
	return nil
}

func pasteActionSameArch(action, srcType, src, dstType, dst string, rename bool, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error {
	klog.Infoln("Now deal with ", action, " for same arch ", dstType)
	klog.Infoln("src: ", src, ", dst: ", dst)

	handler, err := drives.GetResourceService(srcType)
	if err != nil {
		return err
	}

	return handler.PasteSame(action, src, dst, rename, fileCache, w, r)
}

func pasteActionDiffArch(ctx context.Context, action, srcType, src, dstType, dst string, d *common.Data, fileCache fileutils.FileCache, w http.ResponseWriter, r *http.Request) error {
	// In this function, context if tied up to src, because src is in the URL
	switch action {
	case "copy":
		return doPaste(files.DefaultFs, srcType, src, dstType, dst, d, w, r)
	case "rename":
		err := doPaste(files.DefaultFs, srcType, src, dstType, dst, d, w, r)
		if err != nil {
			return err
		}

		handler, err := drives.GetResourceService(srcType)
		if err != nil {
			return err
		}
		err = handler.MoveDelete(fileCache, src, ctx, d, w, r)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported action %s: %w", action, errors.ErrInvalidRequestParams)
	}
	return nil
}
