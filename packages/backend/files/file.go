package files

import (
	"bytes"
	"crypto/md5"  //nolint:gosec
	"crypto/sha1" //nolint:gosec
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/afero"

	"github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/rules"
)

// FileInfo describes a file.
type FileInfo struct {
	*Listing
	Fs           afero.Fs          `json:"-"`
	Path         string            `json:"path"`
	Name         string            `json:"name"`
	Size         int64             `json:"size"`
	FileSize     int64             `json:"fileSize"`
	Extension    string            `json:"extension"`
	ModTime      time.Time         `json:"modified"`
	Mode         os.FileMode       `json:"mode"`
	IsDir        bool              `json:"isDir"`
	IsSymlink    bool              `json:"isSymlink"`
	Type         string            `json:"type"`
	Subtitles    []string          `json:"subtitles,omitempty"`
	Content      string            `json:"content,omitempty"`
	Checksums    map[string]string `json:"checksums,omitempty"`
	Token        string            `json:"token,omitempty"`
	ExternalType string            `json:"externalType,omitempty"`
}

// FileOptions are the options when getting a file info.
type FileOptions struct {
	Fs         afero.Fs
	Path       string
	Modify     bool
	Expand     bool
	ReadHeader bool
	Token      string
	Checker    rules.Checker
	Content    bool
}

var TerminusdHost = os.Getenv("TERMINUSD_HOST")
var ExternalPrefix = os.Getenv("EXTERNAL_PREFIX")

func CheckPath(s, prefix, except string) bool {
	// prefix := "/data/External/"

	if prefix == "" || except == "" {
		return false
	}

	if !strings.HasPrefix(s, prefix) {
		return false
	}

	remaining := s[len(prefix):]

	if strings.HasSuffix(remaining, except) {
		remaining = remaining[:len(remaining)-len(except)]
	}

	return !strings.Contains(remaining, except) // "/")
}

type Response struct {
	Code    int        `json:"code"`
	Data    []DiskInfo `json:"data"`
	Message string     `json:"message"`
}

type DiskInfo struct {
	Path              string  `json:"path"`
	Fstype            string  `json:"fstype"`
	Total             int64   `json:"total"`
	Free              int64   `json:"free"`
	Used              int64   `json:"used"`
	UsedPercent       float64 `json:"usedPercent"`
	InodesTotal       int64   `json:"inodesTotal"`
	InodesUsed        int64   `json:"inodesUsed"`
	InodesFree        int64   `json:"inodesFree"`
	InodesUsedPercent float64 `json:"inodesUsedPercent"`
}

func FetchDiskInfo(url string, header http.Header) ([]DiskInfo, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	//for key, value := range header {
	//	req.Header.Set(key, value)
	//}
	req.Header = header

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	if response.Code != 200 {
		return nil, fmt.Errorf("error code received: %d", response.Code)
	}

	return response.Data, nil
}

func GetExternalType(filePath string, usbData []DiskInfo, hddData []DiskInfo) string {
	fileName := strings.TrimPrefix(strings.TrimSuffix(filePath, "/"), "/")
	lastSlashIndex := strings.LastIndex(fileName, "/")
	if lastSlashIndex != -1 {
		fileName = fileName[lastSlashIndex+1:]
	}

	for _, usb := range usbData {
		if usb.Path == fileName {
			return "mountable"
		}
	}

	for _, hdd := range hddData {
		if hdd.Path == fileName {
			return "hdd"
		}
	}

	return "others"
}

func UnmountUSBIncluster(r *http.Request, usbPath string) (map[string]interface{}, error) {
	url := "http://" + TerminusdHost + "/command/umount-usb-incluster"

	headers := r.Header.Clone()
	headers.Set("Content-Type", "application/json")
	headers.Set("X-Signature", "temp_signature")

	mountPath := strings.TrimPrefix(strings.TrimSuffix(usbPath, "/"), "/")
	lastSlashIndex := strings.LastIndex(mountPath, "/")
	if lastSlashIndex != -1 {
		mountPath = mountPath[lastSlashIndex+1:]
	}

	bodyData := map[string]string{
		"path": mountPath,
	}
	fmt.Println("bodyData:", bodyData)
	body, err := json.Marshal(bodyData)
	if err != nil {
		return nil, err
	}
	fmt.Println("body (byte slice):", body)
	fmt.Println("body (string):", string(body))

	client := &http.Client{}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header = headers
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var responseMap map[string]interface{}
	err = json.Unmarshal(respBody, &responseMap)
	if err != nil {
		return nil, err
	}

	return responseMap, nil
}

// NewFileInfo creates a File object from a path and a given user. This File
// object will be automatically filled depending on if it is a directory
// or a file. If it's a video file, it will also detect any subtitles.
func NewFileInfo(opts FileOptions) (*FileInfo, error) {
	if !opts.Checker.Check(opts.Path) {
		return nil, os.ErrPermission
	}

	file, err := stat(opts)
	if err != nil {
		return nil, err
	}

	if opts.Expand {
		if file.IsDir {
			if err := file.readListing(opts.Checker, opts.ReadHeader); err != nil { //nolint:govet
				return nil, err
			}
			return file, nil
		}

		err = file.detectType(opts.Modify, opts.Content, true)
		if err != nil {
			return nil, err
		}
	}

	return file, err
}

func NewFileInfoWithDiskInfo(opts FileOptions, usbData, hddData []DiskInfo) (*FileInfo, error) {
	if !opts.Checker.Check(opts.Path) {
		return nil, os.ErrPermission
	}

	file, err := stat(opts)
	if err != nil {
		return nil, err
	}

	if opts.Expand {
		if file.IsDir {
			if err := file.readListingWithDiskInfo(opts.Checker, opts.ReadHeader, usbData, hddData); err != nil { //nolint:govet
				return nil, err
			}
			return file, nil
		}

		err = file.detectType(opts.Modify, opts.Content, true)
		if err != nil {
			return nil, err
		}
	}

	return file, err
}

func stat(opts FileOptions) (*FileInfo, error) {
	var file *FileInfo

	if lstaterFs, ok := opts.Fs.(afero.Lstater); ok {
		info, _, err := lstaterFs.LstatIfPossible(opts.Path)
		if err != nil {
			return nil, err
		}
		file = &FileInfo{
			Fs:        opts.Fs,
			Path:      opts.Path,
			Name:      info.Name(),
			ModTime:   info.ModTime(),
			Mode:      info.Mode(),
			IsDir:     info.IsDir(),
			IsSymlink: IsSymlink(info.Mode()),
			Size:      info.Size(),
			Extension: filepath.Ext(info.Name()),
			Token:     opts.Token,
		}
	}

	// regular file
	if file != nil && !file.IsSymlink {
		return file, nil
	}

	// fs doesn't support afero.Lstater interface or the file is a symlink
	info, err := opts.Fs.Stat(opts.Path)
	if err != nil {
		// can't follow symlink
		if file != nil && file.IsSymlink {
			return file, nil
		}
		return nil, err
	}

	// set correct file size in case of symlink
	if file != nil && file.IsSymlink {
		file.Size = info.Size()
		file.IsDir = info.IsDir()
		return file, nil
	}

	file = &FileInfo{
		Fs:        opts.Fs,
		Path:      opts.Path,
		Name:      info.Name(),
		ModTime:   info.ModTime(),
		Mode:      info.Mode(),
		IsDir:     info.IsDir(),
		Size:      info.Size(),
		Extension: filepath.Ext(info.Name()),
		Token:     opts.Token,
	}

	return file, nil
}

// Checksum checksums a given File for a given User, using a specific
// algorithm. The checksums data is saved on File object.
func (i *FileInfo) Checksum(algo string) error {
	if i.IsDir {
		return errors.ErrIsDirectory
	}

	if i.Checksums == nil {
		i.Checksums = map[string]string{}
	}

	reader, err := i.Fs.Open(i.Path)
	if err != nil {
		return err
	}
	defer reader.Close()

	var h hash.Hash

	//nolint:gosec
	switch algo {
	case "md5":
		h = md5.New()
	case "sha1":
		h = sha1.New()
	case "sha256":
		h = sha256.New()
	case "sha512":
		h = sha512.New()
	default:
		return errors.ErrInvalidOption
	}

	_, err = io.Copy(h, reader)
	if err != nil {
		return err
	}

	i.Checksums[algo] = hex.EncodeToString(h.Sum(nil))
	return nil
}

func (i *FileInfo) RealPath() string {
	if realPathFs, ok := i.Fs.(interface {
		RealPath(name string) (fPath string, err error)
	}); ok {
		realPath, err := realPathFs.RealPath(i.Path)
		if err == nil {
			return realPath
		}
	}

	return i.Path
}

// TODO: use constants
//
//nolint:goconst
func (i *FileInfo) detectType(modify, saveContent, readHeader bool) error {
	if IsNamedPipe(i.Mode) {
		i.Type = "blob"
		return nil
	}
	// failing to detect the type should not return error.
	// imagine the situation where a file in a dir with thousands
	// of files couldn't be opened: we'd have immediately
	// a 500 even though it doesn't matter. So we just log it.

	mimetype := mime.TypeByExtension(i.Extension)
	//fmt.Println("extension mimitype:", mimetype)

	var buffer []byte
	if readHeader && mimetype == "" {
		buffer = i.readFirstBytes()
		mimetype = http.DetectContentType(buffer)
		fmt.Println("header mimitype:", mimetype)
	}

	switch {
	case strings.HasPrefix(mimetype, "video"):
		i.Type = "video"
		i.detectSubtitles()
		return nil
	case strings.HasPrefix(mimetype, "audio"):
		i.Type = "audio"
		return nil
	case strings.HasPrefix(mimetype, "image"):
		i.Type = "image"
		return nil
	case strings.HasSuffix(mimetype, "pdf"):
		i.Type = "pdf"
		return nil
	case (strings.HasPrefix(mimetype, "text") || !isBinary(buffer)) && i.Size <= 10*1024*1024: // 10 MB
		i.Type = "text"

		if !modify {
			i.Type = "textImmutable"
		}

		if saveContent {
			afs := &afero.Afero{Fs: i.Fs}
			content, err := afs.ReadFile(i.Path)
			if err != nil {
				return err
			}

			i.Content = string(content)
		}
		return nil
	default:
		i.Type = "blob"
	}

	return nil
}

func (i *FileInfo) readFirstBytes() []byte {
	reader, err := i.Fs.Open(i.Path)
	if err != nil {
		log.Print(err)
		i.Type = "blob"
		return nil
	}
	defer reader.Close()

	buffer := make([]byte, 512) //nolint:gomnd
	n, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		log.Print(err)
		i.Type = "blob"
		return nil
	}

	return buffer[:n]
}

func (i *FileInfo) detectSubtitles() {
	if i.Type != "video" {
		return
	}

	i.Subtitles = []string{}
	ext := filepath.Ext(i.Path)

	// detect multiple languages. Base*.vtt
	// TODO: give subtitles descriptive names (lang) and track attributes
	parentDir := strings.TrimRight(i.Path, i.Name)
	dir, err := afero.ReadDir(i.Fs, parentDir)
	if err == nil {
		base := strings.TrimSuffix(i.Name, ext)
		for _, f := range dir {
			if !f.IsDir() && strings.HasPrefix(f.Name(), base) && strings.HasSuffix(f.Name(), ".vtt") {
				i.Subtitles = append(i.Subtitles, path.Join(parentDir, f.Name()))
			}
		}
	}
}

//func (i *FileInfo) readListing(checker rules.Checker, readHeader bool) error {
//	afs := &afero.Afero{Fs: i.Fs}
//	dir, err := afs.ReadDir(i.Path)
//	if err != nil {
//		return err
//	}
//
//	listing := &Listing{
//		Items:    []*FileInfo{},
//		NumDirs:  0,
//		NumFiles: 0,
//	}
//
//	for _, f := range dir {
//		name := f.Name()
//		fPath := path.Join(i.Path, name)
//
//		if !checker.Check(fPath) {
//			continue
//		}
//
//		isSymlink, isInvalidLink := false, false
//		if IsSymlink(f.Mode()) {
//			isSymlink = true
//			// It's a symbolic link. We try to follow it. If it doesn't work,
//			// we stay with the link information instead of the target's.
//			info, err := i.Fs.Stat(fPath)
//			if err == nil {
//				f = info
//			} else {
//				isInvalidLink = true
//			}
//		}
//
//		file := &FileInfo{
//			Fs:        i.Fs,
//			Name:      name,
//			Size:      f.Size(),
//			ModTime:   f.ModTime(),
//			Mode:      f.Mode(),
//			IsDir:     f.IsDir(),
//			IsSymlink: isSymlink,
//			Extension: filepath.Ext(name),
//			Path:      fPath,
//		}
//
//		if file.IsDir {
//			listing.NumDirs++
//		} else {
//			listing.NumFiles++
//
//			if isInvalidLink {
//				file.Type = "invalid_link"
//			} else {
//				err := file.detectType(true, false, readHeader)
//				if err != nil {
//					return err
//				}
//			}
//		}
//
//		listing.Items = append(listing.Items, file)
//	}
//
//	i.Listing = listing
//	return nil
//}

func (i *FileInfo) readListing(checker rules.Checker, readHeader bool) error {
	afs := &afero.Afero{Fs: i.Fs}
	dir, err := afs.ReadDir(i.Path)
	if err != nil {
		return err
	}

	listing := &Listing{
		Items:         []*FileInfo{},
		NumDirs:       0,
		NumFiles:      0,
		NumTotalFiles: 0,
		Size:          0,
		FileSize:      0,
	}

	for _, f := range dir {
		name := f.Name()
		fPath := path.Join(i.Path, name)

		if !checker.Check(fPath) {
			continue
		}

		isSymlink, isInvalidLink := false, false
		if IsSymlink(f.Mode()) {
			isSymlink = true
			info, err := i.Fs.Stat(fPath)
			if err == nil {
				f = info
			} else {
				isInvalidLink = true
			}
		}

		file := &FileInfo{
			Fs:        i.Fs,
			Name:      name,
			Size:      f.Size(),
			ModTime:   f.ModTime(),
			Mode:      f.Mode(),
			IsDir:     f.IsDir(),
			IsSymlink: isSymlink,
			Extension: filepath.Ext(name),
			Path:      fPath,
		}

		if file.IsDir {
			// err := file.readListing(checker, readHeader)
			// if err != nil {
			// 	return err
			// }
			listing.NumDirs++
			// listing.Size += file.Size + file.Listing.Size
			// listing.NumTotalFiles += file.Listing.NumTotalFiles
		} else {
			listing.NumFiles++
			// listing.NumTotalFiles++

			if isInvalidLink {
				file.Type = "invalid_link"
			} else {
				err := file.detectType(true, false, readHeader)
				if err != nil {
					return err
				}
			}

			listing.Size += file.Size
			listing.FileSize += file.Size
		}

		listing.Items = append(listing.Items, file)
	}

	i.Listing = listing
	return nil
}

func (i *FileInfo) readListingWithDiskInfo(checker rules.Checker, readHeader bool, usbData, hddData []DiskInfo) error {
	afs := &afero.Afero{Fs: i.Fs}
	dir, err := afs.ReadDir(i.Path)
	if err != nil {
		return err
	}

	listing := &Listing{
		Items:         []*FileInfo{},
		NumDirs:       0,
		NumFiles:      0,
		NumTotalFiles: 0,
		Size:          0,
		FileSize:      0,
	}

	for _, f := range dir {
		name := f.Name()
		fPath := path.Join(i.Path, name)

		if !checker.Check(fPath) {
			continue
		}

		isSymlink, isInvalidLink := false, false
		if IsSymlink(f.Mode()) {
			isSymlink = true
			info, err := i.Fs.Stat(fPath)
			if err == nil {
				f = info
			} else {
				isInvalidLink = true
			}
		}

		file := &FileInfo{
			Fs:        i.Fs,
			Name:      name,
			Size:      f.Size(),
			ModTime:   f.ModTime(),
			Mode:      f.Mode(),
			IsDir:     f.IsDir(),
			IsSymlink: isSymlink,
			Extension: filepath.Ext(name),
			Path:      fPath,
		}

		if file.IsDir {
			if CheckPath(file.Path, ExternalPrefix, "/") {
				file.ExternalType = GetExternalType(file.Path, usbData, hddData)
			}
			// err := file.readListing(checker, readHeader)
			// if err != nil {
			// 	return err
			// }
			listing.NumDirs++
			// listing.Size += file.Size + file.Listing.Size
			// listing.NumTotalFiles += file.Listing.NumTotalFiles
		} else {
			listing.NumFiles++
			// listing.NumTotalFiles++

			if isInvalidLink {
				file.Type = "invalid_link"
			} else {
				err := file.detectType(true, false, readHeader)
				if err != nil {
					return err
				}
			}

			listing.Size += file.Size
			listing.FileSize += file.Size
		}

		listing.Items = append(listing.Items, file)
	}

	i.Listing = listing
	return nil
}
