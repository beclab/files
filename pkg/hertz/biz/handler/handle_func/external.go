package handle_func

import (
	"bytes"
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/global"
	"files/pkg/hertz/biz/model/api/external"
	"files/pkg/models"
	"files/pkg/redisutils"
	"fmt"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"k8s.io/klog/v2"
)

func MountPathIncluster(c *app.RequestContext, req external.MountReq) (map[string]interface{}, error) {
	externalType := req.ExternalType
	var urls []string
	if externalType == "smb" {
		urls = []string{"http://" + files.TerminusdHost + "/command/v2/mount-samba", "http://" + files.TerminusdHost + "/command/mount-samba"}
	} else {
		return nil, fmt.Errorf("Unsupported external type: %s", externalType)
	}

	for _, url := range urls {
		bodyStruct := struct {
			SmbPath  string `json:"smbPath"`
			User     string `json:"user"`
			Password string `json:"password"`
		}{
			SmbPath:  req.SmbPath,
			User:     req.User,
			Password: req.Password,
		}

		bodyBytes, err := json.Marshal(bodyStruct)
		if err != nil {
			return nil, err
		}

		client := &http.Client{}
		request, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, err
		}
		c.Request.Header.VisitAll(func(key []byte, value []byte) {
			request.Header.Set(string(key), string(value))
		})
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Signature", "temp_signature")

		resp, err := client.Do(request)
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
			klog.Errorf("Failed to mount by %s to %s", url, files.TerminusdHost)
			klog.Infof("response status: %d, response body: %v", resp.Status, responseMap)
			continue
		}

		return responseMap, nil
	}
	return nil, fmt.Errorf("failed to mount samba")
}

func UnmountPathIncluster(c *app.RequestContext, req external.UnmountReq, path string) (map[string]interface{}, error) {
	externalType := req.ExternalType
	var url = ""
	if externalType == "usb" {
		url = "http://" + files.TerminusdHost + "/command/umount-usb-incluster"
	} else if externalType == "smb" {
		url = "http://" + files.TerminusdHost + "/command/umount-samba-incluster"
	} else {
		return nil, fmt.Errorf("Unsupported external type: %s", externalType)
	}
	klog.Infoln("path:", path)
	klog.Infoln("externalTYpe:", externalType)
	klog.Infoln("url:", url)

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
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	c.Request.Header.VisitAll(func(key []byte, value []byte) {
		request.Header.Set(string(key), string(value))
	})
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Signature", "temp_signature")
	resp, err := client.Do(request)
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

func ResourceMountedHandler(c *app.RequestContext, _ interface{}, _ string) ([]byte, int, error) {
	global.GlobalMounted.Updated()
	return common.ToBytes(map[string]interface{}{
		"code":         0,
		"message":      "success",
		"mounted_data": global.GlobalMounted.GetMountedData(),
	}), consts.StatusOK, nil
}

func ResourceMountHandler(c *app.RequestContext, req interface{}, _ string) ([]byte, int, error) {
	respJson, err := MountPathIncluster(c, req.(external.MountReq))
	if err != nil {
		return nil, common.ErrToStatus(err), err
	}

	if int(respJson["code"].(float64)) != consts.StatusOK {
		klog.Warningf(respJson["message"].(string))
		if strings.Contains(respJson["message"].(string), "mount error(13)") {
			respJson["message"] = "Incorrect username or password"
		}
		if strings.Contains(respJson["message"].(string), "mount error(113)") {
			respJson["message"] = "Unable to find suitable address"
		}
		if strings.Contains(respJson["message"].(string), "mount error(115)") {
			respJson["message"] = "Cannot connect to samba server"
		}
	}

	global.GlobalMounted.Updated()
	return common.ToBytes(respJson), consts.StatusOK, nil
}

func ResourceUnmountHandler(c *app.RequestContext, req interface{}, prefix string) ([]byte, int, error) {
	var p = string(c.Path())
	var path = strings.TrimPrefix(p, prefix)
	if path == "" {
		return nil, consts.StatusBadRequest, errors.New("path invalid")
	}

	var owner = string(c.GetHeader(common.REQUEST_HEADER_OWNER))
	if owner == "" {
		return nil, consts.StatusBadRequest, errors.New("user not found")
	}
	var fileParam, err = models.CreateFileParam(owner, path)
	if err != nil {
		return nil, consts.StatusBadRequest, err
	}

	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return nil, consts.StatusBadRequest, err
	}
	urlPath := uri + fileParam.Path

	file, err := files.NewFileInfo(files.FileOptions{
		Fs:         files.DefaultFs,
		Path:       strings.TrimPrefix(urlPath, "/data"),
		Modify:     true,
		Expand:     false,
		ReadHeader: true,
	})
	if err != nil {
		return nil, common.ErrToStatus(err), err
	}

	respJson, err := UnmountPathIncluster(c, req.(external.UnmountReq), file.Path)
	if err != nil {
		return nil, common.ErrToStatus(err), err
	}

	global.GlobalMounted.Updated()
	return common.ToBytes(respJson), consts.StatusOK, nil
}

func SmbHistoryGetHandler(c *app.RequestContext, _ interface{}, _ string) ([]byte, int, error) {
	bflName := string(c.GetHeader("X-Bfl-User"))
	if bflName == "" {
		return nil, consts.StatusBadRequest, errors.New("missing X-Bfl-User header")
	}

	key := bflName + "_smb_history"

	zset, err := redisutils.RedisClient.ZRevRangeWithScores(key, 0, -1).Result()
	if err != nil {
		return nil, common.ErrToStatus(err), fmt.Errorf("get reverse range with scores from zset failed: %v", err)
	}

	var result []map[string]interface{}

	for _, entry := range zset {
		member := entry.Member.(string)
		score := entry.Score

		hashKey := key + "_url_details:" + member
		var urlInfo map[string]string
		urlInfo, err = redisutils.RedisClient.HGetAll(hashKey).Result()
		if err != nil {
			return nil, common.ErrToStatus(err), err
		}

		item := map[string]interface{}{
			"url":       urlInfo["url"],
			"username":  urlInfo["username"],
			"password":  urlInfo["password"],
			"timestamp": score,
		}

		result = append(result, item)
	}

	return common.ToBytes(result), consts.StatusOK, nil
}

type SMBHistoryData struct {
	URL      string `json:"url"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

func SmbHistoryPutHandler(c *app.RequestContext, req interface{}, _ string) ([]byte, int, error) {
	bflName := string(c.GetHeader("X-Bfl-User"))
	if bflName == "" {
		return nil, consts.StatusBadRequest, errors.New("missing X-Bfl-User header")
	}

	key := bflName + "_smb_history"

	requestData := req.(external.PutSmbHistoryReq).Data

	score := float64(time.Now().Unix())
	for _, datum := range requestData {
		err := redisutils.RedisClient.ZAdd(key, redis.Z{Score: score, Member: datum.URL}).Err()
		if err != nil {
			klog.Errorln("add new member to zset failed: ", err)
			return nil, common.ErrToStatus(err), err
		}

		hashKey := key + "_url_details:" + datum.URL

		var fields = map[string]interface{}{
			"url":      datum.URL,
			"username": datum.Username,
			"password": datum.Password,
		}
		for field, value := range fields {
			_, err = redisutils.RedisClient.HSet(hashKey, field, value).Result()
			if err != nil {
				klog.Errorf("set hash field '%s' failed: %v\n", field, err)
				return nil, common.ErrToStatus(err), err
			}
		}
	}

	return []byte("Successfully added/updated SMB history and hash"), consts.StatusOK, nil
}

func SmbHistoryDeleteHandler(c *app.RequestContext, req interface{}, _ string) ([]byte, int, error) {
	bflName := string(c.GetHeader("X-Bfl-User"))
	if bflName == "" {
		return nil, consts.StatusBadRequest, errors.New("missing X-Bfl-User header")
	}

	key := bflName + "_smb_history"

	requestData := req.(external.DeleteSmbHistoryReq).Data

	var urls []string
	for _, datum := range requestData {
		urls = append(urls, datum.URL)

		hashKey := key + "_url_details:" + datum.URL
		_, err := redisutils.RedisClient.Del(hashKey).Result()
		if err != nil {
			klog.Errorf("Delete key failed: %v\n", err)
			return nil, common.ErrToStatus(err), err
		}
	}

	err := redisutils.RedisClient.ZRem(key, urls).Err()
	if err != nil {
		klog.Errorln("remove member for zset failed: ", err)
		return nil, common.ErrToStatus(err), err
	}

	return []byte("Successfully deleted SMB history"), consts.StatusOK, nil
}
