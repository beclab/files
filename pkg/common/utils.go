package common

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"k8s.io/klog/v2"
)

var (
	userFirst = "abcdefghijklmnopqrstuvwxyz"
	userRest  = "abcdefghijklmnopqrstuvwxyz0123456789"

	lower   = "abcdefghijklmnopqrstuvwxyz"
	upper   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digits  = "0123456789"
	special = "-_"
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

func IsDiskSpaceEnough(diskSize, fileSize int64) (int64, bool) {
	requiredSpace := int64(float64(fileSize) * 1.05)
	return requiredSpace, diskSize < requiredSpace
}

func RemoveBlank(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

func GenerateTaskId() string {
	var n = time.Now()
	var id = n.UnixMicro()
	return fmt.Sprintf("task%d", id)
}

func SplitNameExt(filename string) (name, ext string) {
	for _, e := range multiExts {
		if strings.HasSuffix(filename, e) {
			return filename[:len(filename)-len(e)], e
		}
	}
	idx := strings.LastIndex(filename, ".")
	if idx > 0 {
		return filename[:idx], filename[idx:]
	}
	return filename, ""
}

func GenerateAccount() (string, string) {
	var u, p string
	var err error
	for {
		u, err = genUsername()
		if err != nil {
			continue
		}
		p, err = genPassword()
		if err != nil {
			continue
		}

		break
	}

	return u, p
}

func rint(n int) (int, error) {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(v.Int64()), nil
}

func rchar(set string) (byte, error) {
	i, err := rint(len(set))
	if err != nil {
		return 0, err
	}
	return set[i], nil
}

func shuffle(b []byte) error {
	for i := len(b) - 1; i > 0; i-- {
		j, err := rint(i + 1)
		if err != nil {
			return err
		}
		b[i], b[j] = b[j], b[i]
	}
	return nil
}

func genUsername() (string, error) {
	nDelta, err := rint(5) // 0..4
	if err != nil {
		return "", err
	}
	n := 8 + nDelta

	out := make([]byte, n)
	ch, err := rchar(userFirst)
	if err != nil {
		return "", err
	}
	out[0] = ch
	for i := 1; i < n; i++ {
		ch, err := rchar(userRest)
		if err != nil {
			return "", err
		}
		out[i] = ch
	}

	if out[n-1] == '-' {
		out[n-1] = '_'
	}
	return string(out), nil
}

func genPassword() (string, error) {
	n := 16
	pool := lower + upper + digits + special

	out := make([]byte, 0, n)

	req := []string{lower, upper, digits, special}
	for _, set := range req {
		ch, err := rchar(set)
		if err != nil {
			return "", err
		}
		out = append(out, ch)
	}

	for len(out) < n {
		ch, err := rchar(pool)
		if err != nil {
			return "", err
		}
		out = append(out, ch)
	}

	if err := shuffle(out); err != nil {
		return "", err
	}
	return string(out), nil
}

func EscapeGlob(s string) string {
	if s == "" {
		return s
	}

	specials := map[rune]bool{
		'\\': true,
		'*':  true,
		'?':  true,
		'[':  true,
		']':  true,
		'{':  true,
		'}':  true,
		'(':  true,
		')':  true,
		'|':  true,
		'^':  true,
		'+':  true,
		'.':  true,
		'$':  true,
	}

	var b strings.Builder
	b.Grow(len(s) * 2)

	for _, r := range s {
		if specials[r] {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
