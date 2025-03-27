package common

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	libErrors "files/pkg/errors"
	"fmt"
	"io"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
)

var RootPrefix = os.Getenv("ROOT_PREFIX")

var BflCookieCache = make(map[string]string)

func init() {
	if RootPrefix == "" {
		RootPrefix = "/data"
	}
}

func Md5File(filepath string) (string, error) {
	if !strings.HasPrefix(filepath, RootPrefix) {
		filepath = RootPrefix + filepath
	}

	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	buf := make([]byte, 8192)

	for {
		n, err := file.Read(buf)
		if n > 0 {
			hash.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func Md5String(s string) string {
	hasher := md5.New()
	hasher.Write([]byte(s))
	return hex.EncodeToString(hasher.Sum(nil))
}

func ErrToStatus(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case os.IsPermission(err):
		return http.StatusForbidden
	case os.IsNotExist(err), err == libErrors.ErrNotExist:
		return http.StatusNotFound
	case os.IsExist(err), err == libErrors.ErrExist:
		return http.StatusConflict
	case errors.Is(err, libErrors.ErrPermissionDenied):
		return http.StatusForbidden
	case errors.Is(err, libErrors.ErrInvalidRequestParams):
		return http.StatusBadRequest
	case errors.Is(err, libErrors.ErrRootUserDeletion):
		return http.StatusForbidden
	case err.Error() == "file size exceeds 4GB":
		return http.StatusRequestEntityTooLarge
	default:
		return http.StatusInternalServerError
	}
}

func RenderJSON(w http.ResponseWriter, _ *http.Request, data interface{}) (int, error) {
	marsh, err := json.Marshal(data)

	if err != nil {
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write(marsh); err != nil {
		return http.StatusInternalServerError, err
	}

	return 0, nil
}

func GetHost(bflName string) string {
	hostUrl := "http://bfl.user-space-" + bflName + "/bfl/info/v1/terminus-info"

	resp, err := http.Get(hostUrl)
	if err != nil {
		klog.Errorln("Error making GET request:", err)
		return ""
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		klog.Errorln("Error reading response body:", err)
		return ""
	}

	if resp.StatusCode != http.StatusOK {
		klog.Infof("Received non-200 response: %d\n", resp.StatusCode)
		return ""
	}

	type BflResponse struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			TerminusName    string `json:"terminusName"`
			WizardStatus    string `json:"wizardStatus"`
			Selfhosted      bool   `json:"selfhosted"`
			TailScaleEnable bool   `json:"tailScaleEnable"`
			OsVersion       string `json:"osVersion"`
			LoginBackground string `json:"loginBackground"`
			Avatar          string `json:"avatar"`
			TerminusId      string `json:"terminusId"`
			Did             string `json:"did"`
			ReverseProxy    string `json:"reverseProxy"`
			Terminusd       string `json:"terminusd"`
		} `json:"data"`
	}

	var responseObj BflResponse
	err = json.Unmarshal(body, &responseObj)
	if err != nil {
		klog.Errorln("Error unmarshaling JSON:", err)
		return ""
	}

	modifiedTerminusName := strings.Replace(responseObj.Data.TerminusName, "@", ".", 1)
	klog.Infoln(modifiedTerminusName)
	return "https://files." + modifiedTerminusName
}

const (
	maxReasonableSpace = 1000 * 1e12 // 1000T
)

func CheckDiskSpace(filePath string, newContentSize int64) (bool, int64, int64, int64, error) {
	reservedSpaceStr := os.Getenv("RESERVED_SPACE") // env is MB, default is 10000MB
	if reservedSpaceStr == "" {
		reservedSpaceStr = "10000"
	}
	reservedSpace, err := strconv.ParseInt(reservedSpaceStr, 10, 64)
	if err != nil {
		return false, 0, 0, 0, fmt.Errorf("failed to parse reserved space: %w", err)
	}
	reservedSpace *= 1024 * 1024

	var rootStat, dataStat syscall.Statfs_t

	err = syscall.Statfs("/", &rootStat)
	if err != nil {
		return false, 0, 0, 0, fmt.Errorf("failed to get root file system stats: %w", err)
	}
	rootAvailableSpace := int64(rootStat.Bavail * uint64(rootStat.Bsize))

	err = syscall.Statfs(filePath, &dataStat)
	if err != nil {
		klog.Error(err)
		return false, 0, 0, 0, fmt.Errorf("failed to get /data file system stats: %w", err)
	}
	dataAvailableSpace := int64(dataStat.Bavail * uint64(dataStat.Bsize))

	availableSpace := int64(0)
	if dataAvailableSpace >= maxReasonableSpace {
		availableSpace = rootAvailableSpace - reservedSpace
	} else {
		availableSpace = dataAvailableSpace - reservedSpace
	}

	requiredSpace := newContentSize

	if availableSpace >= requiredSpace {
		return true, requiredSpace, availableSpace, reservedSpace, nil
	}

	return false, requiredSpace, availableSpace, reservedSpace, nil
}

func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	var result string
	var value float64

	if bytes >= GB {
		value = float64(bytes) / GB
		result = fmt.Sprintf("%.4fG", value)
	} else if bytes >= MB {
		value = float64(bytes) / MB
		result = fmt.Sprintf("%.4fM", value)
	} else if bytes >= KB {
		value = float64(bytes) / KB
		result = fmt.Sprintf("%.4fK", value)
	} else {
		result = strconv.FormatInt(bytes, 10) + "B"
	}

	return result
}

func RemoveSlash(s string) string {
	s = strings.TrimSuffix(s, "/")
	return strings.ReplaceAll(s, "/", "_")
}
