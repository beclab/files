package seahub

import (
	"bytes"
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/sync/seahub/seaserv"
	"files/pkg/models"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/chai2010/tiff" // Register TIFF decoder
	"github.com/disintegration/imaging"
	"github.com/rwcarlsen/goexif/exif"
	_ "golang.org/x/image/webp" // Register WEBP decoder
	"k8s.io/klog/v2"
)

const (
	THUMBNAIL_EXTENSION                 = ".png"
	THUMBNAIL_IMAGE_SIZE_LIMIT          = 30
	THUMBNAIL_IMAGE_ORIGINAL_SIZE_LIMIT = 10.0 // MB
)

func GetThumbnail(fileParam *models.FileParam, previewSize string) ([]byte, error) {
	repoId := fileParam.Extend

	repo, err := seaserv.GlobalSeafileAPI.GetRepo(repoId)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	if repo == nil {
		klog.Errorf("repo %s not found", repoId)
		return nil, errors.New("repo not found")
	}

	sizeStr := strings.Trim(previewSize, "/")
	if sizeStr == "" {
		return nil, errors.New("size is missing")
	}

	size, err := strconv.Atoi(sizeStr)
	if err != nil {
		klog.Errorf("Invalid size parameter: %v", err)
		return nil, err
	}

	path := filepath.Clean(fileParam.Path)
	fileId, err := seaserv.GlobalSeafileAPI.GetFileIdByPath(repoId, path)
	if err != nil {
		return nil, errors.New("internal server error")
	}
	if fileId == "" {
		klog.Errorf("file %s not found", path)
		return nil, errors.New("file not found")
	}

	encrypted, err := strconv.ParseBool(repo["encrypted"])
	if err != nil {
		klog.Errorf("Error parsing repo encrypted: %v", err)
		encrypted = false
	}
	if encrypted {
		return nil, errors.New("Permission denied.")
	}

	username := fileParam.Owner + "@auth.local"

	permission, err := CheckFolderPermission(username, repoId, path)
	if err != nil || permission != "rw" {
		return nil, errors.New("permission denied")
	}

	success, statusCode := generateThumbnail(username, repoId, strconv.Itoa(size), path)
	if success {
		thumbnailDir := filepath.Join(THUMBNAIL_ROOT, strconv.Itoa(size))
		_, thumbext := common.SplitNameExt(fileParam.Path)
		thumbext = strings.ToLower(thumbext)
		if !(thumbext == ".jpg" || thumbext == ".jpeg" || thumbext == ".png") {
			thumbext = THUMBNAIL_EXTENSION
		}
		thumbnailFile := filepath.Join(thumbnailDir, fileId+thumbext)

		content, err := os.ReadFile(thumbnailFile)
		if err != nil {
			klog.Errorf("Failed to read thumbnail: %v", err)
			return nil, err
		}

		return content, nil
	}

	switch statusCode {
	case 400:
		return nil, errors.New("Invalid argument")
	case 403:
		return nil, errors.New("Forbidden")
	case 500:
		return nil, errors.New("Failed to generate thumbnail.")
	default:
		return nil, errors.New("Unknown error")
	}
}

func generateThumbnail(username, repoId string, sizeStr string, path string) (bool, int) {
	size, err := strconv.Atoi(sizeStr)
	if err != nil {
		klog.Errorf("Invalid size parameter: %v", err)
		return false, http.StatusBadRequest
	}

	thumbnailDir := filepath.Join(THUMBNAIL_ROOT, strconv.Itoa(size))
	if err := os.MkdirAll(thumbnailDir, 0755); err != nil {
		klog.Errorf("Failed to create directory: %v", err)
		return false, http.StatusInternalServerError
	}

	filetype, fileext := getFileTypeAndExt(filepath.Base(path))

	if filetype == VIDEO && !ENABLE_VIDEO_THUMBNAIL {
		return false, http.StatusBadRequest
	}

	fileId, err := seaserv.GlobalSeafileAPI.GetFileIdByPath(repoId, path)
	if err != nil {
		return false, http.StatusBadRequest
	}
	if fileId == "" {
		klog.Errorf("file %s not found", path)
		return false, http.StatusNotFound
	}

	thumbnailFile := filepath.Join(thumbnailDir, fileId+"."+fileext)
	if _, err := os.Stat(thumbnailFile); err == nil {
		return true, http.StatusOK
	}

	repo, err := seaserv.GlobalSeafileAPI.GetRepo(repoId)
	if err != nil {
		return false, http.StatusBadRequest
	}
	if repo == nil {
		return false, http.StatusNotFound
	}

	version, err := strconv.Atoi(repo["version"])
	if err != nil {
		klog.Error(err)
		return false, http.StatusBadRequest
	}
	fileSize, err := seaserv.GlobalSeafileAPI.GetFileSize(repo["store_id"], version, fileId)
	if err != nil {
		klog.Error(err)
		return false, http.StatusBadRequest
	}

	switch filetype {
	case VIDEO:
		if !ENABLE_VIDEO_THUMBNAIL {
			return false, http.StatusBadRequest
		}
	default:
		if fileSize > THUMBNAIL_IMAGE_SIZE_LIMIT*1024*1024 {
			return false, http.StatusBadRequest
		}

		if fileext == "psd" {
			return createPSDThumbnails(repo, fileId, path, size, thumbnailFile, fileSize)
		}

		tmpFile, err := os.CreateTemp("", "img-*.tmp")
		if err != nil {
			klog.Errorf("Create temp file failed: %v", err)
			return false, http.StatusInternalServerError
		}
		defer os.Remove(tmpFile.Name())
		defer tmpFile.Close()

		token, err := seaserv.GlobalSeafileAPI.GetFileServerAccessToken(repo["id"], fileId, "view", "", true)
		if err != nil {
			klog.Error(err)
			return false, http.StatusInternalServerError
		}
		if token == "" {
			return false, http.StatusInternalServerError
		}

		innerPath := GenFileGetURL(token, filepath.Base(path))
		resp, err := http.Get("http://127.0.0.1:80/" + innerPath)
		if err != nil {
			klog.Errorf("Download failed: %v", err)
			return false, http.StatusBadRequest
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			klog.Errorf("Unexpected status: %s", resp.Status)
			return false, http.StatusBadRequest
		}

		if _, err = io.Copy(tmpFile, resp.Body); err != nil {
			klog.Errorf("Save temp file failed: %v", err)
			return false, http.StatusInternalServerError
		}

		return createThumbnailCommon(tmpFile.Name(), thumbnailFile, size)
	}
	return false, http.StatusBadRequest
}

func createPSDThumbnails(repo map[string]string, fileId, path string, size int, thumbnailFile string, fileSize int64) (bool, int) {
	startTime := time.Now()
	defer func() {
		fmt.Printf("Extract psd image [%s](size: %d) takes: %v\n", path, fileSize, time.Since(startTime))
	}()

	psdTmp, err := os.CreateTemp("", "psd-*.tmp")
	if err != nil {
		fmt.Printf("Create PSD temp file error: %v\n", err)
		return false, 500
	}
	defer os.Remove(psdTmp.Name())
	defer psdTmp.Close()

	token, err := seaserv.GlobalSeafileAPI.GetFileServerAccessToken(repo["id"], fileId, "view", "", false)
	if err != nil {
		klog.Error(err)
		return false, http.StatusInternalServerError
	}
	if token == "" {
		return false, http.StatusInternalServerError
	}

	innerPath := GenFileGetURL(token, filepath.Base(path))
	resp, err := http.Get("http://127.0.0.1:80/" + innerPath)
	if err != nil {
		fmt.Printf("Download error: %v\n", err)
		return false, 500
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("HTTP error: %s\n", resp.Status)
		return false, 500
	}

	if _, err := io.Copy(psdTmp, resp.Body); err != nil {
		fmt.Printf("Save PSD temp file error: %v\n", err)
		return false, 500
	}

	pngTmp, err := os.CreateTemp("", "png-*.tmp")
	if err != nil {
		fmt.Printf("Create PNG temp file error: %v\n", err)
		return false, 500
	}
	defer os.Remove(pngTmp.Name())
	defer pngTmp.Close()

	cmd := exec.Command("convert", psdTmp.Name()+"[0]", pngTmp.Name())
	if err := cmd.Run(); err != nil {
		fmt.Printf("PSD conversion error: %v\n", err)
		return false, 500
	}

	return createThumbnailCommon(pngTmp.Name(), thumbnailFile, size)
}

func createThumbnailCommon(srcFile, thumbnailFile string, size int) (bool, int) {
	fileInfo, err := os.Stat(srcFile)
	if err != nil {
		klog.Errorf("Source file error: %v", err)
		return false, http.StatusBadRequest
	}

	klog.Infof("File Analysis Report for: %s", srcFile)
	klog.Infof("1. File Size: %d bytes (%.2f MB)",
		fileInfo.Size(), float64(fileInfo.Size())/1024/1024)

	fileHeader := make([]byte, 512)
	if hf, err := os.Open(srcFile); err == nil {
		if _, err := hf.Read(fileHeader); err != nil {
			klog.Warningf("2. Header read failed: %v", err)
		}
		hf.Close()
	}

	fileType := detectFileType(fileHeader)
	klog.Infof("2. Detected format: %s", fileType)

	contentType := http.DetectContentType(fileHeader)
	klog.Infof("3. MIME type: %s", contentType)

	var img image.Image
	if img, err = ImageDecode(srcFile); err != nil {
		klog.Errorf("4. Image decode test failed: %v", err)
		return false, http.StatusBadRequest
	} else {
		klog.Infof("4. Image dimensions: %dx%d",
			img.Bounds().Dx(), img.Bounds().Dy())
	}

	thumbDir := filepath.Dir(thumbnailFile)
	if err := os.MkdirAll(thumbDir, 0755); err != nil {
		klog.Errorf("Failed to create thumbnail directory: %v", err)
		return false, http.StatusInternalServerError
	}

	// exif section, can be deleted for base usage
	var exifData *exif.Exif
	if exifData, err = ExifDecode(srcFile); err != nil {
		klog.Warningf("5. EXIF decode failed: %v", err)
	} else if exifData != nil {
		klog.Infof("5. EXIF data found")
	} else {
		klog.Infof("5. No EXIF data")
	}

	if exifData != nil {
		if tag, err := exifData.Get(exif.Orientation); err == nil {
			switch tag.String() {
			case "2":
				img = imaging.FlipH(img)
			case "3":
				img = imaging.Rotate180(img)
			case "4":
				img = imaging.FlipV(img)
			case "5":
				img = imaging.Transpose(img)
			case "6":
				img = imaging.Rotate270(img)
			case "7":
				img = imaging.Transverse(img)
			case "8":
				img = imaging.Rotate90(img)
			}
		}
	}

	thumb := imaging.Thumbnail(img, size, size, imaging.Lanczos)
	if thumb == nil {
		klog.Errorf("Thumbnail generation failed: nil image returned")
		return false, http.StatusInternalServerError
	}

	_, ext := common.SplitNameExt(thumbnailFile)
	ext = strings.ToLower(ext)
	var saveErr error
	switch ext {
	case ".jpg", ".jpeg":
		saveErr = imaging.Save(thumb, thumbnailFile, imaging.JPEGQuality(90))
	case ".png":
		saveErr = imaging.Save(thumb, thumbnailFile, imaging.PNGCompressionLevel(6))
	default:
		saveErr = imaging.Save(thumb, strings.TrimSuffix(thumbnailFile, ext)+THUMBNAIL_EXTENSION, imaging.PNGCompressionLevel(6))
		klog.Warningf("Unknown format %s, saved as PNG", ext)
	}

	if saveErr != nil {
		klog.Errorf("Save thumbnail failed: %v (file=%s)", saveErr, thumbnailFile)
		return false, http.StatusInternalServerError
	}

	return true, http.StatusOK
}

func detectFileType(header []byte) string {
	// JPEG: FF D8 FF
	if len(header) >= 3 && header[0] == 0xFF && header[1] == 0xD8 && header[2] == 0xFF {
		return "JPEG"
	}

	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if len(header) >= 8 && bytes.Equal(header[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return "PNG"
	}

	// GIF: 47 49 46 38
	if len(header) >= 4 && bytes.Equal(header[:4], []byte{'G', 'I', 'F', '8'}) {
		return "GIF"
	}

	// PDF: 25 50 44 46
	if len(header) >= 4 && bytes.Equal(header[:4], []byte{0x25, 0x50, 0x44, 0x46}) {
		return "PDF"
	}

	// ZIP-based format（eg. DOCX）: 50 4B 03 04
	if len(header) >= 4 && bytes.Equal(header[:4], []byte{0x50, 0x4B, 0x03, 0x04}) {
		return "ZIP-based"
	}

	if len(header) >= 12 &&
		bytes.Equal(header[0:4], []byte{'R', 'I', 'F', 'F'}) &&
		bytes.Equal(header[8:12], []byte{'W', 'E', 'B', 'P'}) {
		return "WEBP"
	}

	// TIFF (little-endian): 49 49 2A 00
	if len(header) >= 4 && bytes.Equal(header[:4], []byte{0x49, 0x49, 0x2A, 0x00}) {
		return "TIFF"
	}

	// TIFF (big-endian): 4D 4D 00 2A
	if len(header) >= 4 && bytes.Equal(header[:4], []byte{0x4D, 0x4D, 0x00, 0x2A}) {
		return "TIFF"
	}

	return "Unknown"
}

func ImageDecode(filePath string) (image.Image, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open failed: %v", err)
	}
	defer file.Close()

	if _, format, err := image.DecodeConfig(file); err != nil {
		return nil, fmt.Errorf("pre-decode check failed: %v", err)
	} else {
		klog.Infof("Detected format: %s", format) // 记录实际格式
	}

	if _, err := file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("file reset failed: %v", err)
	}

	img, format, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode failed (format: %s): %v", format, err)
	}
	return img, nil
}

func ExifDecode(filePath string) (*exif.Exif, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open failed: %v", err)
	}
	defer file.Close()

	if _, err = file.Seek(0, 0); err != nil {
		klog.Errorf("Reset file pointer error: %v", err)
		return nil, err
	}

	exifData, err := exif.Decode(file)
	if err != nil {
		return nil, err
	}
	return exifData, nil
}
