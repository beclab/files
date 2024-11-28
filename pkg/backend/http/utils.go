package http

import (
	"encoding/json"
	"errors"
	libErrors "files/pkg/backend/errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

func renderJSON(w http.ResponseWriter, _ *http.Request, data interface{}) (int, error) {
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

func errToStatus(err error) int {
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

// This is an addaptation if http.StripPrefix in which we don't
// return 404 if the page doesn't have the needed prefix.
func stripPrefix(prefix string, h http.Handler) http.Handler {
	if prefix == "" || prefix == "/" {
		return h
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, prefix)
		rp := strings.TrimPrefix(r.URL.RawPath, prefix)
		r2 := new(http.Request)
		*r2 = *r
		r2.URL = new(url.URL)
		*r2.URL = *r.URL
		r2.URL.Path = p
		r2.URL.RawPath = rp
		h.ServeHTTP(w, r2)
	})
}

const (
	maxReasonableSpace = 1000 * 1e12 // 1000T
)

func checkDiskSpace(filePath string, newContentSize int64) (bool, int64, int64, int64, error) {
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
		fmt.Println(err)
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

func formatBytes(bytes int64) string {
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

func removeSlash(s string) string {
	s = strings.TrimSuffix(s, "/")
	return strings.ReplaceAll(s, "/", "_")
}

func getHost(r *http.Request) string {
	bflName := r.Header.Get("X-Bfl-User")
	hostUrl := "http://bfl.user-space-" + bflName + "/bfl/info/v1/terminus-info"

	resp, err := http.Get(hostUrl)
	if err != nil {
		fmt.Println("Error making GET request:", err)
		return ""
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return ""
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Received non-200 response: %d\n", resp.StatusCode)
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
		fmt.Println("Error unmarshaling JSON:", err)
		return ""
	}

	modifiedTerminusName := strings.Replace(responseObj.Data.TerminusName, "@", ".", 1)
	fmt.Println(modifiedTerminusName)
	return "https://files." + modifiedTerminusName
}

func getOwner(r *http.Request) (ownerID, ownerName string) {
	bflName := r.Header.Get("X-Bfl-User")
	url := "http://bfl.user-space-" + bflName + "/bfl/info/v1/terminus-info"

	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Error making GET request:", err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Received non-200 response: %d\n", resp.StatusCode)
		return
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
		fmt.Println("Error unmarshaling JSON:", err)
		return
	}

	ownerID = responseObj.Data.TerminusId
	ownerName = responseObj.Data.TerminusName
	return
}

func isURLEscaped(s string) bool {
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

func unescapeURLIfEscaped(s string) (string, error) {
	var result = s
	var err error
	if isURLEscaped(s) {
		result, err = url.QueryUnescape(s)
		if err != nil {
			return "", err
		}
	}
	return result, nil
}

func escapeURLWithSpace(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

func escapeAndJoin(input string, delimiter string) string {
	segments := strings.Split(input, delimiter)
	for i, segment := range segments {
		segments[i] = escapeURLWithSpace(segment)
	}
	return strings.Join(segments, delimiter)
}
