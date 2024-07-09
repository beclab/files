package rpc

import (
	"encoding/json"
	"fmt"
	"github.com/filebrowser/filebrowser/v2/my_redis"
	"os"
	"strings"
	"time"
)

//func GetSearchFolderStatus() (map[string]interface{}, error) {
//	indexingStatus, err := strconv.Atoi(my_redis.RedisGet("indexing_status"))
//	if err != nil {
//		fmt.Println(err)
//		return nil, err
//	}
//	indexingError, err := strconv.ParseBool(my_redis.RedisGet("indexing_error"))
//	if err != nil {
//		fmt.Println(err)
//		return nil, err
//	}
//	var status = "indexing"
//	if indexingError {
//		status = "errored"
//	} else {
//		if indexingStatus == 0 {
//			status = "running"
//		}
//	}
//	count, err := RpcServer.EsCountFiles(FileIndex)
//	if err != nil {
//		fmt.Println(err)
//		return nil, err
//	}
//	result := map[string]interface{}{
//		"status":           status,
//		"last_update_time": my_redis.RedisGet("last_update_time"),
//		"index_doc_num":    count,
//		"paths":            my_redis.RedisGet("paths"),
//	}
//	return result, nil
//}

func isInvalidDir(dirPath string) bool {
	// 使用 os.Stat 函数获取路径的文件信息
	fileInfo, err := os.Stat(dirPath)

	if err != nil {
		// 如果出现错误，表示路径不存在或无法访问
		if os.IsNotExist(err) {
			fmt.Printf("目录 %s 不存在\n", dirPath)
		} else {
			fmt.Printf("无法访问目录 %s：%v\n", dirPath, err)
		}
		return true
	}

	// 检查路径是否为目录
	if fileInfo.IsDir() {
		fmt.Printf("%s 是一个目录\n", dirPath)
		return false
	} else {
		fmt.Printf("%s 不是一个目录\n", dirPath)
		return true
	}
}

func isSubdir(subPath string, path string) bool {
	return strings.HasPrefix(subPath, path) && strings.HasPrefix(strings.TrimPrefix(subPath, path), "/")
}

func dedupArray(paths []string, prefix string) []string {
	// 使用 map 实现去重
	uniqueMap := make(map[string]bool)
	for _, path := range paths {
		if isInvalidDir(path) {
			continue
		}

		if prefix != "" && !isSubdir(path, prefix) {
			continue
		}
		uniqueMap[path] = true
	}

	// 将 map 中的键转换回数组
	uniquePaths := make([]string, 0, len(uniqueMap))
	for path := range uniqueMap {
		uniquePaths = append(uniquePaths, path)
	}

	var result []string
	for _, subPath := range uniquePaths {
		nodup := true
		for _, path := range uniquePaths {
			if (path != subPath) && isSubdir(subPath, path) {
				nodup = false
				break
			}
		}
		if nodup {
			result = append(result, subPath)
		}
	}
	//fmt.Println("redis paths: ", result)
	return result
}

//// difference 函数返回仅在 a 中存在而不在 b 中存在的元素切片
//func difference(a, b []string) []string {
//	m := make(map[string]bool)
//	for _, value := range b {
//		m[value] = true
//	}
//
//	var diff []string
//	for _, value := range a {
//		if !m[value] {
//			diff = append(diff, value)
//		}
//	}
//
//	return diff
//}
//
//func UpdateSearchFolderPaths(paths []string) {
//	// A为原始，B为输入，设A、B均经历过“父子去重”，自己“父子去重”为简单去重后，取出所有没有在自己中有任一元素作为父祖路径的元素集合
//	// 则：
//	// ADD = B-A
//	// REDIS = B
//	// DELETE = A-B
//
//	// A
//	agoPaths := dedupArray(strings.Split(my_redis.RedisGet("paths"), ","), PathPrefix)
//	fmt.Println("ago paths: ", agoPaths, len(agoPaths))
//
//	// B
//	basePaths := dedupArray(paths, PathPrefix)
//	fmt.Println("base paths: ", basePaths, len(basePaths))
//
//	// ADD
//	addPaths := difference(basePaths, agoPaths)
//	fmt.Println("add paths: ", addPaths, len(addPaths))
//
//	// REDIS
//	redisPaths := basePaths // append(addPaths, calibPaths...)
//	fmt.Println("redis paths:", redisPaths, len(redisPaths))
//
//	// DELETE
//	deletePaths := difference(agoPaths, redisPaths)
//	fmt.Println("delete paths:", deletePaths, len(deletePaths))
//
//	my_redis.RedisSet("paths", strings.Join(redisPaths, ","), time.Duration(0))
//	WatchPath(addPaths, deletePaths)
//	return
//}

type DatasetRedis struct {
	DatasetID      string   `json:"datasetID"`
	Paths          []string `json:"paths"`
	LastUpdateTime string   `json:"lastUpdateTime"`
}

func UpdateDatasetFolderPaths(datasetID string, paths []string) {
	//getRedisKey := fmt.Sprintf("user-space-%s_zinc-files:DATASET_%s", BflName, datasetID)
	getRedisKey := fmt.Sprintf("DATASET_%s", datasetID)
	setRedisKey := fmt.Sprintf("DATASET_%s", datasetID)

	// 从 Redis 中获取数据集信息
	datasetJSON := my_redis.RedisGet(getRedisKey)

	var dataset DatasetRedis
	if datasetJSON != "" {
		// 解析 Redis 中的数据集信息
		err := json.Unmarshal([]byte(datasetJSON), &dataset)
		if err != nil {
			fmt.Printf("解析 Redis 中的数据集信息失败：%s\n", err.Error())
			return
		}
	}

	// 更新数据集的路径信息
	dataset.DatasetID = datasetID
	fmt.Println("paths=", paths)
	fmt.Println("PathPrefix=", PathPrefix)
	dataset.Paths = dedupArray(paths, PathPrefix)
	fmt.Println("dataset.Paths=", dataset.Paths)
	dataset.LastUpdateTime = fmt.Sprintf("%d", time.Now().Unix())

	fmt.Println("DatasetID:", dataset.DatasetID, ", Paths:", dataset.Paths, ", LastUpdateTime:", dataset.LastUpdateTime)

	// 将更新后的数据集信息存储回 Redis
	newDatasetJSON, err := json.Marshal(dataset)
	if err != nil {
		fmt.Printf("序列化更新后的数据集信息失败：%s\n", err.Error())
		return
	}
	fmt.Println(newDatasetJSON)

	my_redis.RedisSet(setRedisKey, string(newDatasetJSON), time.Duration(0))
}
