package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/filebrowser/filebrowser/v2/common"
	"github.com/filebrowser/filebrowser/v2/my_redis"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func isPhotoPath(filepath string) bool {
	// 使用strings.SplitN将URL按"/"分割成最多4个部分（包括空字符串）
	parts := strings.SplitN(filepath, "/", 4)

	// 检查分割后的数组长度是否足够
	if len(parts) < 4 {
		return false // URL不符合要求
	}

	// 从第三个"/"开始及以后的部分（即parts的最后一个元素）
	suffix := "/" + parts[3]

	// 检查这部分是否以目标前缀开头
	return strings.HasPrefix(suffix, PhotosPath)
}

// extractSegment 函数提取URL中第n和第n+1个斜杠之间的部分
func extractSegment(filepath string, n int) (string, bool) {
	// 使用strings.Split将URL按"/"分割成多个部分
	parts := strings.Split(filepath, "/")

	// 检查分割后的数组长度是否足够
	if len(parts) <= n {
		return "", false // URL不符合要求，没有足够的部分
	}

	// 返回第n和第n+1个斜杠之间的部分
	return parts[n], true
}

func checkOrUpdatePhotosRedis(filepath, fileMd5 string, op int) error {
	if PhotosEnabled != "True" {
		return nil
	}

	// op: 1, add; 2, upload; 3, delete
	if !isPhotoPath(filepath) {
		log.Debug().Msg(filepath + " is not a photo. Skip it.")
		return nil
	}

	log.Debug().Msg("Dealing with photo " + filepath)

	machineName, success := extractSegment(filepath, 5)
	if !success {
		log.Debug().Msg(filepath + " deoesn't have a machine name.")
		return nil
	}
	hashName := "PHOTOS_" + machineName
	filepathMd5 := common.Md5String(filepath)

	if fileMd5 == "" {
		f, err := os.Open(filepath)
		if err != nil {
			return err
		}
		b, err := ioutil.ReadAll(f)
		f.Close()
		if err != nil {
			return err
		}
		fileMd5 = common.Md5File(bytes.NewReader(b))
	}

	var err error
	switch op {
	case 1: // add
		err = addOrUploadPhotoRedis(hashName, filepath, filepathMd5, fileMd5, false)
		if err != nil {
			return err
		}
	case 2: // upload
		err = addOrUploadPhotoRedis(hashName, filepath, filepathMd5, fileMd5, true)
		if err != nil {
			return err
		}
		//err = markPhotoAsUploaded(hashName, filepathMd5, true)
		//if err != nil {
		//	return err
		//}
	case 3: // delete
		err = markPhotoAsDeleted(hashName, filepathMd5, true)
		if err != nil {
			return err
		}
	default:
		log.Warn().Msgf("Unknown operation type: %d", op)
	}

	return nil
}

func addOrUploadPhotoRedis(hashName, filepath, filepathMd5, fileMd5 string, uploading bool) error {
	redisValue := my_redis.RedisHGet(hashName, filepathMd5)

	// only response for add when no redis value yet
	if redisValue == "" {
		if !uploading {
			photoObject := map[string]interface{}{
				"md5":      fileMd5,
				"uploaded": false,
				"deleted":  false,
			}

			my_redis.RedisHSet(hashName, filepathMd5, photoObject)

			log.Debug().Msgf("Added new photo entry for %s in Redis", filepath)
		}
	} else {
		// can response for add and upload when redis value existed, both updating md5 (given warning if not match)
		var redisData map[string]interface{}
		err := json.Unmarshal([]byte(redisValue), &redisData)
		if err != nil {
			log.Error().Msgf("Failed to unmarshal Redis data for file %s: %v", filepath, err)
			return err
		}
		// uploaded only can be updated (to true) only when it is false and uploading is true
		if !redisData["uploaded"].(bool) && uploading {
			redisData["uploaded"] = true
		}

		redisMd5, ok := redisData["md5"].(string)
		if !ok || redisMd5 != fileMd5 {
			log.Warn().Msgf("MD5 mismatch for file %s: Redis MD5=%s, File MD5=%s", filepath, redisMd5, fileMd5)

			redisData["md5"] = fileMd5
			newData, err := json.Marshal(redisData)
			if err != nil {
				log.Error().Msgf("Failed to marshal updated Redis data for file %s: %v", filepath, err)
				return err
			}

			my_redis.RedisHSet(hashName, filepathMd5, string(newData))

			log.Debug().Msgf("Updated MD5 for file %s in Redis", filepath)
		}
	}
	return nil
}

//func markPhotoAsUploaded(hashName, filepathMd5 string, status bool) error {
//	redisValue := my_redis.RedisHGet(hashName, filepathMd5)
//	if redisValue == "" {
//		log.Warn().Msgf("No entry found for %s in Redis when marking as uploaded", filepathMd5)
//		return nil
//	}
//
//	var redisData map[string]interface{}
//	err := json.Unmarshal([]byte(redisValue), &redisData)
//	if err != nil {
//		return fmt.Errorf("failed to unmarshal Redis data: %v", err)
//	}
//
//	redisData["uploaded"] = status
//	newData, err := json.Marshal(redisData)
//	if err != nil {
//		return fmt.Errorf("failed to marshal updated Redis data: %v", err)
//	}
//
//	my_redis.RedisHSet(hashName, filepathMd5, string(newData))
//
//	log.Debug().Msgf("Marked photo %s as uploaded in Redis", filepathMd5)
//	return nil
//}

func markPhotoAsDeleted(hashName, filepathMd5 string, status bool) error {
	redisValue := my_redis.RedisHGet(hashName, filepathMd5)
	if redisValue == "" {
		log.Warn().Msgf("No entry found for %s in Redis when marking as deleted", filepathMd5)
		return nil
	}

	var redisData map[string]interface{}
	err := json.Unmarshal([]byte(redisValue), &redisData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal Redis data: %v", err)
	}

	redisData["deleted"] = status
	newData, err := json.Marshal(redisData)
	if err != nil {
		return fmt.Errorf("failed to marshal updated Redis data: %v", err)
	}

	my_redis.RedisHSet(hashName, filepathMd5, string(newData))

	log.Debug().Msgf("Marked photo %s as deleted in Redis", filepathMd5)
	return nil
}

type Photo struct {
	Filename string `json:"filename"`
	MD5      string `json:"md5"`
}

type PreCheckRequest struct {
	DeviceName string  `json:"device_name"`
	Photos     []Photo `json:"photos"`
}

func (s *Service) preCheckHandler(c *gin.Context) {
	bflName := c.GetHeader("X-Bfl-User")
	pvc, err := BflPVCs.getUserPVCOrCache(bflName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}

	var req PreCheckRequest

	// 解析请求 JSON
	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	basePath := "/data/" + pvc + "/Home/Pictures/" + req.DeviceName

	// 检查并创建 device_name 目录
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		err = os.MkdirAll(basePath, 0755)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create directory"})
			return
		}
	}

	var notStoredPaths []string

	// 遍历照片信息
	for _, photo := range req.Photos {
		// 构建完整的文件路径
		filePath := filepath.Join(basePath, photo.Filename)

		// 获取文件所在的目录
		dirPath := filepath.Dir(filePath)

		// 检查并创建目录
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			err = os.MkdirAll(dirPath, 0755)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create directory for " + filePath})
				return
			}
		}

		// 检查文件是否存在
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			notStoredPaths = append(notStoredPaths, filePath)
		}

		// 插入 Redis 记录（这里假设我们只需要设置一次，且设置一个过期时间）
		err = checkOrUpdatePhotosRedis(filePath, photo.MD5, 1)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update Redis for " + filePath})
			return
		}
	}

	// 返回没有存储的图片路径（如果需要的话，也可以返回其他信息）
	c.JSON(http.StatusOK, gin.H{
		"not_stored_paths": notStoredPaths,
		// 可以添加其他字段，比如 "message": "Pre-check completed"
	})
}
