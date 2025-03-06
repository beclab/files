package rpc

import (
	"encoding/json"
	"files/pkg/backend/common"
	"files/pkg/backend/redisutils"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"github.com/rs/zerolog/log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func isPhotoPath(filepath string) bool {
	parts := strings.SplitN(filepath, "/", 4)

	if len(parts) < 4 {
		return false
	}

	suffix := "/" + parts[3]

	return strings.HasPrefix(suffix, PhotosPath)
}

func extractSegment(filepath string, n int) (string, bool) {
	parts := strings.Split(filepath, "/")

	if len(parts) <= n {
		return "", false
	}

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
		var err error
		fileMd5, err = common.Md5File(filepath)
		if err != nil {
			return err
		}
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
	redisValue, err := redisutils.RedisClient.HGet(hashName, filepathMd5).Result()
	if err != nil {
		if err != redis.Nil {
			fmt.Println("get key value of Hash table failed: ", err)
			return err
		}
		fmt.Println("Hash table ", hashName, " and field ", filepathMd5, "doesn't exist")
		return err
	}

	// only response for add when no redis value yet
	if redisValue == "" {
		if !uploading {
			photoObject := map[string]interface{}{
				"md5":      fileMd5,
				"uploaded": false,
				"deleted":  false,
			}

			err = redisutils.RedisClient.HSet(hashName, filepathMd5, photoObject).Err()
			if err != nil {
				fmt.Println("Set key value of Hash table failed: ", err)
				return err
			}

			log.Debug().Msgf("Added new photo entry for %s in Redis", filepath)
		}
	} else {
		// can response for add and upload when redis value existed, both updating md5 (given warning if not match)
		var redisData map[string]interface{}
		err = json.Unmarshal([]byte(redisValue), &redisData)
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
			var newData []byte
			newData, err = json.Marshal(redisData)
			if err != nil {
				log.Error().Msgf("Failed to marshal updated Redis data for file %s: %v", filepath, err)
				return err
			}

			err = redisutils.RedisClient.HSet(hashName, filepathMd5, string(newData)).Err()
			if err != nil {
				fmt.Println("Set key value of Hash table failed: ", err)
				return err
			}

			log.Debug().Msgf("Updated MD5 for file %s in Redis", filepath)
		}
	}
	return nil
}

func markPhotoAsDeleted(hashName, filepathMd5 string, status bool) error {
	redisValue, err := redisutils.RedisClient.HGet(hashName, filepathMd5).Result()
	if err != nil {
		if err != redis.Nil {
			fmt.Println("get key value of Hash table failed: ", err)
			return err
		}
		fmt.Println("Hash table ", hashName, " and field ", filepathMd5, "doesn't exist")
		return err
	}
	if redisValue == "" {
		log.Warn().Msgf("No entry found for %s in Redis when marking as deleted", filepathMd5)
		return nil
	}

	var redisData map[string]interface{}
	err = json.Unmarshal([]byte(redisValue), &redisData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal Redis data: %v", err)
	}

	redisData["deleted"] = status
	newData, err := json.Marshal(redisData)
	if err != nil {
		return fmt.Errorf("failed to marshal updated Redis data: %v", err)
	}

	err = redisutils.RedisClient.HSet(hashName, filepathMd5, string(newData)).Err()
	if err != nil {
		fmt.Println("Set key value of Hash table failed: ", err)
		return err
	}

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

	err = c.BindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	basePath := "/data/" + pvc + "/Home/Pictures/" + req.DeviceName

	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		err = os.MkdirAll(basePath, 0755)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create directory"})
			return
		}
	}

	var notStoredPaths []string

	for _, photo := range req.Photos {
		filePath := filepath.Join(basePath, photo.Filename)

		dirPath := filepath.Dir(filePath)

		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			err = os.MkdirAll(dirPath, 0755)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create directory for " + filePath})
				return
			}
		}

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			notStoredPaths = append(notStoredPaths, filePath)
		}

		err = checkOrUpdatePhotosRedis(filePath, photo.MD5, 1)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update Redis for " + filePath})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"not_stored_paths": notStoredPaths,
	})
}
