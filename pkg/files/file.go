package files

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"files/pkg/common"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/klog/v2"

	"github.com/spf13/afero"
)

var DefaultFs = afero.NewBasePathFs(afero.NewOsFs(), os.Getenv("ROOT_PREFIX"))
var DefaultSorting = Sorting{
	By:  "name",
	Asc: true,
}

// FileInfo describes a file.
type FileInfo struct {
	*Listing
	Fs           afero.Fs          `json:"-"`
	FsType       string            `json:"fileType"`
	FsExtend     string            `json:"fileExtend"`
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
	ReadOnly     *bool             `json:"readOnly,omitempty"`
}

func (fi *FileInfo) String() string {
	res, err := json.Marshal(fi)
	if err != nil {
		return ""
	}
	return string(res)
}

// FileOptions are the options when getting a file info.
type FileOptions struct {
	Fs         afero.Fs
	FsType     string
	FsExtend   string
	Path       string
	Modify     bool
	Expand     bool
	ReadHeader bool
	Token      string
	Content    bool
}

var TerminusdHost = os.Getenv("TERMINUSD_HOST")
var ExternalPrefix = os.Getenv("EXTERNAL_PREFIX")

type Response struct {
	Code    int        `json:"code"`
	Data    []DiskInfo `json:"data"`
	Message string     `json:"message"`
}

type DiskInfo struct {
	Type              string  `json:"type"`
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
	ReadOnly          *bool   `json:"read_only,omitempty"`
	Invalid           bool    `json:"invalid"`
	IDSerial          string  `json:"id_serial,omitempty"`
	IDSerialShort     string  `json:"id_serial_short,omitempty"`
	PartitionUUID     string  `json:"partition_uuid,omitempty"`
}

func MountPathIncluster(r *http.Request) (map[string]interface{}, error) {
	externalType := r.URL.Query().Get("external_type")
	var urls []string
	if externalType == "smb" {
		urls = []string{"http://" + TerminusdHost + "/command/v2/mount-samba", "http://" + TerminusdHost + "/command/mount-samba"}
	} else {
		return nil, fmt.Errorf("Unsupported external type: %s", externalType)
	}

	for _, url := range urls {
		bodyBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

		headers := r.Header.Clone()
		headers.Set("Content-Type", "application/json")
		headers.Set("X-Signature", "temp_signature")

		client := &http.Client{}
		req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, err
		}
		req.Header = headers

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp == nil {
			klog.Errorf("not get response from %s", url)
			continue
		}

		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var responseMap map[string]interface{}
		err = json.Unmarshal(respBody, &responseMap)
		if err != nil {
			return nil, err
		}

		err = resp.Body.Close()
		if err != nil {
			return responseMap, err
		}

		if resp.StatusCode >= 400 {
			klog.Errorf("Failed to mount by %s to %s", url, TerminusdHost)
			klog.Infof("response status: %s, response body: %v", resp.Status, responseMap)
			continue
		}

		return responseMap, nil
	}
	return nil, fmt.Errorf("failed to mount samba")
}

func UnmountPathIncluster(r *http.Request, path string) (map[string]interface{}, error) {
	externalType := r.URL.Query().Get("external_type")
	var url = ""
	if externalType == "usb" {
		url = "http://" + TerminusdHost + "/command/umount-usb-incluster"
	} else if externalType == "smb" {
		url = "http://" + TerminusdHost + "/command/umount-samba-incluster"
	} else {
		return nil, fmt.Errorf("Unsupported external type: %s", externalType)
	}
	klog.Infoln("path:", path)
	klog.Infoln("externalTYpe:", externalType)
	klog.Infoln("url:", url)

	headers := r.Header.Clone()
	headers.Set("Content-Type", "application/json")
	headers.Set("X-Signature", "temp_signature")

	mountPath := strings.TrimPrefix(strings.TrimSuffix(path, "/"), "/")
	lastSlashIndex := strings.LastIndex(mountPath, "/")
	if lastSlashIndex != -1 {
		mountPath = mountPath[lastSlashIndex+1:]
	}
	klog.Infoln("mountPath:", mountPath)

	bodyData := map[string]string{
		"path": mountPath,
	}
	klog.Infoln("bodyData:", bodyData)
	body, err := json.Marshal(bodyData)
	if err != nil {
		return nil, err
	}
	klog.Infoln("body (byte slice):", body)
	klog.Infoln("body (string):", string(body))

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
	klog.Infoln("responseMap:", responseMap)

	return responseMap, nil
}

// NewFileInfo creates a File object from a path and a given user. This File
// object will be automatically filled depending on if it is a directory
// or a file. If it's a video file, it will also detect any subtitles.
func NewFileInfo(opts FileOptions) (*FileInfo, error) {
	file, err := stat(opts)
	if err != nil {
		return nil, err
	}

	if opts.Expand {
		if file.IsDir {
			if err := file.readListing(opts.ReadHeader); err != nil {
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
		_, fext := common.SplitNameExt(info.Name())
		file = &FileInfo{
			Fs:        opts.Fs,
			FsType:    opts.FsType,
			FsExtend:  opts.FsExtend,
			Path:      opts.Path,
			Name:      info.Name(),
			ModTime:   info.ModTime(),
			Mode:      info.Mode(),
			IsDir:     info.IsDir(),
			IsSymlink: IsSymlink(info.Mode()),
			Size:      info.Size(),
			Extension: fext,
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

	_, fext := common.SplitNameExt(info.Name())
	file = &FileInfo{
		Fs:        opts.Fs,
		FsType:    opts.FsType,
		FsExtend:  opts.FsExtend,
		Path:      opts.Path,
		Name:      info.Name(),
		ModTime:   info.ModTime(),
		Mode:      info.Mode(),
		IsDir:     info.IsDir(),
		Size:      info.Size(),
		Extension: fext,
		Token:     opts.Token,
	}

	return file, nil
}

// Checksum checksums a given File for a given User, using a specific
// algorithm. The checksums data is saved on File object.
func (i *FileInfo) Checksum(algo string) error {
	if i.IsDir {
		return common.ErrIsDirectory
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
		return common.ErrInvalidOption
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

	var buffer []byte
	if readHeader && mimetype == "" {
		buffer = i.readFirstBytes()
		mimetype = http.DetectContentType(buffer)
		klog.Infoln("header mimitype:", mimetype)
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
		klog.Error(err)
		i.Type = "blob"
		return nil
	}
	defer reader.Close()

	buffer := make([]byte, 512)
	n, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		klog.Error(err)
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
	_, ext := common.SplitNameExt(i.Path)

	// detect multiple languages. Base*.vtt
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

func (i *FileInfo) readDirents() ([]os.FileInfo, error) {
	var (
		dir      []os.FileInfo
		firstErr error
		err      error
	)

	afs := &afero.Afero{Fs: i.Fs}
	dir, err = afs.ReadDir(i.Path)
	if err != nil {
		klog.Error(err)
		if !strings.Contains(err.Error(), "host is down") {
			return nil, err
		}

		err = filepath.WalkDir("/data"+i.Path, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if strings.Contains(err.Error(), "host is down") {
					klog.Warningf("[WARN] skipping %s: %v\n", path, err)
					return nil
				}
				firstErr = fmt.Errorf("walk error %s: %w", path, err)
				return err
			}

			if path == "/data"+i.Path {
				return nil
			}

			relPath, err := filepath.Rel("/data"+i.Path, path)
			if err != nil {
				return nil
			}

			if strings.Count(relPath, "/") > 0 {
				return filepath.SkipDir
			}

			info, err := d.Info()
			if err != nil {
				if strings.Contains(err.Error(), "host is down") {
					klog.Warningf("[WARN] skipping %s: %v\n", path, err)
					return nil
				}
				firstErr = fmt.Errorf("get file info failed %s: %w", path, err)
				return err
			}

			dir = append(dir, info)
			return nil
		})

		if err != nil {
			klog.Errorln(firstErr)
			return nil, err
		}
	}
	return dir, nil
}

func (i *FileInfo) readListing(readHeader bool) error {
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
		if f.IsDir() {
			if !strings.HasSuffix(fPath, "/") {
				fPath += "/"
			}
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

		_, fext := common.SplitNameExt(name)
		file := &FileInfo{
			Fs:        i.Fs,
			FsType:    i.FsType,
			FsExtend:  i.FsExtend,
			Path:      fPath,
			Name:      name,
			Size:      f.Size(),
			ModTime:   f.ModTime(),
			Mode:      f.Mode(),
			IsDir:     f.IsDir(),
			IsSymlink: isSymlink,
			Extension: fext,
		}

		if file.IsDir {
			listing.NumDirs++
		} else {
			listing.NumFiles++

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
