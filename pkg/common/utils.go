package common

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"k8s.io/klog/v2"
)

var RootPrefix = os.Getenv("ROOT_PREFIX")

var BflCookieCache = make(map[string]string)

func init() {
	if RootPrefix == "" {
		RootPrefix = "/data"
	}
}

func MD5FileHeader(fileHeader *multipart.FileHeader) (string, error) {
	start := time.Now()
	klog.Infoln("Function CalculateMD5 starts at", start)
	defer func() {
		elapsed := time.Since(start)
		klog.Infof("Function CalculateMD5 execution time: %v\n", elapsed)
	}()

	// Open the file
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Create an MD5 hash object
	hash := md5.New()

	// Copy the file content to the hash object
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	// Compute the hash and get the checksum
	hashInBytes := hash.Sum(nil)[:16]

	// Convert the byte array to a hexadecimal string
	md5String := hex.EncodeToString(hashInBytes)

	return md5String, nil
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

func ToJson(v any) string {
	r, _ := json.Marshal(v)
	return string(r)
}

func ToBytes(v any) []byte {
	r, _ := json.Marshal(v)
	return r
}

func IsURLEscaped(s string) bool {
	escapePattern := `%[0-9A-Fa-f]{2}`
	re := regexp.MustCompile(escapePattern)

	if re.MatchString(s) {
		decodedStr, err := url.QueryUnescape(s)
		if err != nil {
			return false
		}
		return decodedStr != s
	}
	return false
}

func UnescapeURLIfEscaped(s string) (string, error) {
	var result = s
	var err error
	if IsURLEscaped(s) {
		result, err = url.QueryUnescape(s)
		if err != nil {
			return "", err
		}
	}
	return result, nil
}

func EscapeURLWithSpace(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

func EscapeAndJoin(input string, delimiter string) string {
	segments := strings.Split(input, delimiter)
	for i, segment := range segments {
		segments[i] = EscapeURLWithSpace(segment)
	}
	return strings.Join(segments, delimiter)
}

func PathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}

	if os.IsNotExist(err) {
		return false
	}
	return false
}

// ListContains returns a boolean that v is in items
func ListContains[T comparable](items []T, v T) bool {
	if items == nil {
		return false
	}

	for _, item := range items {
		if v == item {
			return true
		}
	}
	return false
}
