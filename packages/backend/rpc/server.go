package rpc

import (
	"context"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"

	//"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

const (
	Success            = 0
	ErrorCodeUnknow    = -101
	ErrorCodeInput     = -102
	ErrorCodeDelete    = -103
	ErrorCodeUnmarshal = -104
	ErrorCodeTimeout   = -105
)

var SessionCookieName = "session_id"

var Host = "127.0.0.1"

//var ESEnabled = os.Getenv("ES_ENABLED")

var WatcherEnabled = os.Getenv("WATCHER_ENABLED")

var KnowledgeBaseEnabled = os.Getenv("KNOWLEDGE_BASE_ENABLED")

//var FileIndex = os.Getenv("ZINC_INDEX") // "Files"

var PathPrefix = os.Getenv("PATH_PREFIX") // "/Home"

var RootPrefix = os.Getenv("ROOT_PREFIX") // "/data"

var CacheRootPath = os.Getenv("CACHE_ROOT_PATH") // "/appcache"

var ContentPath = os.Getenv("CONTENT_PATH") //	"/Home/Documents"

var BflName = os.Getenv("BFL_NAME")

const DefaultMaxResult = 10

var once sync.Once

var RpcServer *Service

var maxPendingLength = 30

type Service struct {
	port             string
	zincUrl          string
	username         string
	password         string
	esClient         *elasticsearch.TypedClient
	context          context.Context
	maxPendingLength int
	CallbackGroup    *gin.RouterGroup
	KubeConfig       *rest.Config
	k8sClient        *kubernetes.Clientset
}

func InitRpcService(url, port, username, password string, bsModelConfig map[string]string) {
	once.Do(func() {
		//esClient, _ := InitES(url, username, password)
		ctxTemp := context.WithValue(context.Background(), "Username", username)
		ctx := context.WithValue(ctxTemp, "Password", password)

		config := ctrl.GetConfigOrDie()
		k8sClient := kubernetes.NewForConfigOrDie(config)

		RpcServer = &Service{
			port:     port,
			zincUrl:  url,
			username: username,
			password: password,
			//esClient:         esClient,
			context:          ctx,
			maxPendingLength: maxPendingLength,
			KubeConfig:       config,
			k8sClient:        k8sClient,
		}
		PVCs = NewPVCCache(RpcServer)

		//if ESEnabled == "True" {
		//	if err := RpcServer.EsSetupIndex(); err != nil {
		//		panic(err)
		//	}
		//}

		//load routes
		RpcServer.loadRoutes()
		//RpcServer.SendUpdateDatasetFolderPathsRequest(true)
	})
}

type LoggerMy struct {
}

func (*LoggerMy) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	if strings.Index(msg, `"/healthcheck"`) > 0 {
		return
	}
	return
}

type Resp struct {
	ResultCode int    `json:"code"`
	ResultMsg  string `json:"data"`
}

var RpcEngine *gin.Engine

func (c *Service) Start(ctx context.Context) error {
	address := "0.0.0.0:" + c.port
	go RpcEngine.Run(address)
	log.Printf("start rpc on port:%s", c.port)
	return nil
}

func (c *Service) loadRoutes() error {
	//start gin
	gin.DefaultWriter = &LoggerMy{}
	RpcEngine = gin.Default()

	//cors middleware
	RpcEngine.SetTrustedProxies(nil)
	RpcEngine.GET("/files/healthcheck", func(c *gin.Context) {
		fmt.Println("You really see me")
		c.String(http.StatusOK, "ok")
	})

	//RpcEngine.POST("/files/input", c.HandleInput)
	//RpcEngine.POST("/files/delete", c.HandleDelete)
	//RpcEngine.POST("/files/query", c.HandleQuery)
	//RpcEngine.POST("/provider/query_file", c.QueryFile)
	//下面一行用于重定向测试：
	//RpcEngine.POST("/provider/query_file", c.HandleSearchFolderPaths)
	//RpcEngine.POST("/provider/get_search_folder_status", c.HandleSearchFolderStatus)
	//RpcEngine.POST("/provider/update_search_folder_paths", c.HandleSearchFolderPaths)
	//if KnowledgeBaseEnabled == "True" {
	//	RpcEngine.POST("/provider/get_dataset_folder_status", c.HandleDatasetFolderStatus)
	//	RpcEngine.POST("/provider/update_dataset_folder_paths", c.HandleDatasetFolderPaths)
	//	RpcEngine.POST("/api/get_dataset_folder_status_test", c.HandleDatasetFolderStatusTest)
	//	RpcEngine.POST("/api/update_dataset_folder_paths_test", c.HandleDatasetFolderPathsTest)
	//}

	c.CallbackGroup = RpcEngine.Group("/api/callback")
	log.Printf("init rpc server")
	return nil
}

//func (s *Service) HandleInput(c *gin.Context) {
//	index := c.Query("index")
//	if index == "Files" || index == "" {
//		index = FileIndex
//	}
//	if index != FileIndex {
//		rep := Resp{
//			ResultCode: ErrorCodeUnknow,
//			ResultMsg:  fmt.Sprintf("only support index %s", FileIndex),
//		}
//		c.JSON(http.StatusBadRequest, rep)
//	}
//	if index == FileIndex {
//		s.HandleFileInput(c)
//	}
//}

//func (s *Service) HandleDelete(c *gin.Context) {
//	index := c.Query("index")
//	if index == "Files" || index == "" {
//		index = FileIndex
//	}
//	if index != FileIndex {
//		rep := Resp{
//			ResultCode: ErrorCodeUnknow,
//			ResultMsg:  fmt.Sprintf("only support index %s", FileIndex),
//		}
//		c.JSON(http.StatusBadRequest, rep)
//	}
//	if index == FileIndex {
//		s.HandleFileDelete(c)
//	}
//}

//func (s *Service) HandleQuery(c *gin.Context) {
//	index := c.Query("index")
//	if index == "Files" || index == "" {
//		index = FileIndex
//	}
//	if index != FileIndex {
//		rep := Resp{
//			ResultCode: ErrorCodeUnknow,
//			ResultMsg:  fmt.Sprintf("only support index %s", FileIndex),
//		}
//		c.JSON(http.StatusBadRequest, rep)
//	}
//	if index == FileIndex {
//		s.HandleFileQuery(c)
//	}
//}

//func (s *Service) HandleSearchFolderStatus(c *gin.Context) {
//
//	response, err := GetSearchFolderStatus()
//	defer func() {
//	if err == nil {
//		c.JSON(http.StatusOK, response)
//	} else {
//		c.JSON(http.StatusInternalServerError, "Internel server error")
//	}
//	}()
//}
//
//type PathsProviderRequest struct {
//	Op       string        `json:"op"`
//	DataType string        `json:"datatype"`
//	Version  string        `json:"version"`
//	Group    string        `json:"group"`
//	Token    string        `json:"token"`
//	Data     *PathsRequest `json:"data"`
//}
//
//type PathsRequest struct {
//	Paths []string `json:"paths"`
//	//Index  string `json:"index"`
//	//Query []string `json:"query"`
//	//Limit  int    `json:"limit"`
//	//Offset int    `json:"offset"`
//}
//
//func (s *Service) HandleSearchFolderPaths(c *gin.Context) {
//	fmt.Println("You got the files-deployment search folder paths provider~!")
//	token := PathsProviderRequest{
//		Token: c.GetHeader("X-Access-Token"),
//	}
//	if err := c.ShouldBindJSON(&token); err != nil {
//		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse request body"})
//		return
//	}
//
//	fmt.Println("search folder paths")
//	fmt.Println(token)
//	fmt.Println(token.Data)
//
//	req := c.Request
//	// 解析表单数据
//	if err := req.ParseForm(); err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse form data"})
//		return
//	}
//
//	// 添加paths字段
//	var paths = []string{}
//	if len(token.Data.Paths) > 0 {
//		fmt.Println(token.Data.Paths)
//		paths = token.Data.Paths
//	} else {
//		//fmt.Println(token.Data.Query)
//		//paths = token.Data.Query
//	}
//	fmt.Println(paths)
//
//	UpdateSearchFolderPaths(paths)
//	response := map[string]interface{}{
//		"paths": my_redis.RedisGet("paths"),
//	}
//	c.Header("Content-Type", "application/json")
//	defer func() {
//		c.JSON(http.StatusOK, response)
//	}()
//}

//type DatasetFolderStatusProviderRequest struct {
//	Op       string                      `json:"op"`
//	DataType string                      `json:"datatype"`
//	Version  string                      `json:"version"`
//	Group    string                      `json:"group"`
//	Token    string                      `json:"token"`
//	Data     *DatasetFolderStatusRequest `json:"data"`
//}
//
//type DatasetFolderStatusRequest struct {
//	DatasetIDs   []string `json:"dataset_ids"`
//	DatasetNames []string `json:"dataset_names"`
//	Clean        int      `json:"clean"`
//}

//func processDatasetIDs(datasetIDs []string, datasetNames []string) ([]string, error) {
//	type DatasetResponse struct {
//		Data struct {
//			Data []struct {
//				ID   string `json:"id"`
//				Name string `json:"name"`
//			} `json:"data"`
//		} `json:"data"`
//	}
//
//	resp, err := CallDifyGatewayBaseProvider(1, nil)
//	if err != nil {
//		return nil, fmt.Errorf("请求失败：%s\n", err.Error())
//	}
//
//	var datasetResponse DatasetResponse
//	err = json.NewDecoder(bytes.NewReader(resp)).Decode(&datasetResponse)
//	if err != nil {
//		return nil, fmt.Errorf("解析响应失败：%s\n", err.Error())
//	}
//
//	for _, dataItem := range datasetResponse.Data.Data {
//		if contains(datasetNames, dataItem.Name) {
//			key := fmt.Sprintf("DATASET_%s", dataItem.ID)
//			value := my_redis.RedisGet(key)
//			if value != "" {
//				if !contains(datasetIDs, dataItem.ID) {
//					datasetIDs = append(datasetIDs, dataItem.ID)
//				}
//			}
//		}
//	}
//
//	return datasetIDs, nil
//}

//func contains(list []string, element string) bool {
//	for _, item := range list {
//		if item == element {
//			return true
//		}
//	}
//	return false
//}

//type Dataset struct {
//	DatasetID      string   `json:"datasetID"`
//	Paths          []string `json:"Paths"`
//	LastUpdateTime string   `json:"lastUpdateTime"`
//	DatasetName    string   `json:"datasetName"`
//	IndexDocNum    int      `json:"indexDocNum"`
//	Status         string   `json:"status"`
//}
//
//func getRedisDatasetIDs() ([]string, error) {
//	allKeys, _ := my_redis.RedisGetAllKeys()
//	fmt.Println("Current Redis All keys:", allKeys)
//	// 获取以 "DATASET_" 开头的所有 key
//	keys := my_redis.RedisGetKeys("*DATASET_*")
//	fmt.Println("DATASET keys:", keys)
//
//	// 提取 key 中的 datasetID
//	datasetIDs := make([]string, len(keys))
//	for i, key := range keys {
//		parts := strings.Split(key, "_")
//		datasetIDs[i] = parts[len(parts)-1]
//		data := my_redis.RedisGet(key)
//		fmt.Println("Redis key: ", key, ", Data: ", data)
//		data = my_redis.RedisGet("DATASET_" + datasetIDs[i])
//		fmt.Println("Redis key: ", "DATASET_"+datasetIDs[i], ", Data: ", data)
//	}
//	return datasetIDs, nil
//}

//func GetDatasetFolderStatus(datasetIDs []string, all int, clean int) (map[string]interface{}, error) {
//	fmt.Println("You see get dataset folder status function~Pray for it!")
//	type DataItem struct {
//		ID            string `json:"id"`
//		Name          string `json:"name"`
//		DocumentCount int    `json:"document_count"`
//		AppCount      int    `json:"app_count"`
//	}
//
//	type DatasetResponse struct {
//		Data struct {
//			Data []DataItem `json:"data"`
//		} `json:"data"`
//	}
//
//	var err error
//	if len(datasetIDs) == 0 && all == 1 {
//		// 获取 Redis 中以 "DATASET_" 开头的所有 key
//		datasetIDs, err = getRedisDatasetIDs()
//		fmt.Println("datasetIDs:", datasetIDs)
//		if err != nil {
//			fmt.Println(err)
//			return nil, fmt.Errorf("获取 Redis 中的 datasetIDs 失败: %s", err.Error())
//		}
//	}
//
//	var datasetResponse DatasetResponse
//	var dataCopy []DataItem
//	if clean == 0 {
//		resp, err := CallDifyGatewayBaseProvider(1, nil)
//		fmt.Println("resp:", resp)
//		if err != nil {
//			fmt.Println(err)
//			return nil, fmt.Errorf("请求失败：%s\n", err.Error())
//		}
//
//		err = json.NewDecoder(bytes.NewReader(resp)).Decode(&datasetResponse)
//		if err != nil {
//			return nil, fmt.Errorf("解析响应失败：%s\n", err.Error())
//		}
//
//		//fmt.Println(url)
//		fmt.Println("datasetResponse=", datasetResponse)
//
//		// 创建 datasetResponseCopy 的副本
//		dataCopy = make([]DataItem, len(datasetResponse.Data.Data))
//		copy(dataCopy, datasetResponse.Data.Data)
//	} else {
//		dataCopy = []DataItem{}
//	}
//	response := make(map[string]interface{})
//
//	for _, datasetID := range datasetIDs {
//		fmt.Println("datasetID=", datasetID)
//		key := fmt.Sprintf("DATASET_%s", datasetID)
//		value := my_redis.RedisGet(key)
//		if value == "" {
//			continue
//		}
//
//		var dataset map[string]interface{}
//		err = json.Unmarshal([]byte(value), &dataset)
//		if err != nil {
//			return nil, fmt.Errorf("Parse Redis to JSON failed: %s\n", err.Error())
//		}
//
//		fmt.Println("dataCopy=", dataCopy)
//		for _, dataItem := range dataCopy {
//			fmt.Println("dataItem.ID=", dataItem.ID)
//			if dataItem.ID == datasetID {
//				dataset["datasetName"] = dataItem.Name
//				dataset["indexDocNum"] = dataItem.DocumentCount
//				dataset["linkedAgentNum"] = dataItem.AppCount
//				if dataItem.Name == BflName+"'s Document" {
//					dataset["default"] = true
//				} else {
//					dataset["default"] = false
//				}
//				break
//			}
//		}
//
//		if clean == 0 {
//			indexOpData := make(map[string]interface{})
//			indexOpData["datasetID"] = datasetID
//			indexStatusResp, err := CallDifyGatewayBaseProvider(3, indexOpData)
//			if err != nil {
//				dataset["status"] = "error"
//			} else {
//				var indexStatusResponse struct {
//					Data []struct{} `json:"data"`
//				}
//				err = json.NewDecoder(bytes.NewReader(indexStatusResp)).Decode(&datasetResponse)
//				if err != nil {
//					dataset["status"] = "error"
//				} else {
//					if len(indexStatusResponse.Data) == 0 {
//						dataset["status"] = "running"
//					} else {
//						dataset["status"] = "indexing"
//					}
//				}
//			}
//		}
//
//		response[datasetID] = dataset
//	}
//
//	return response, nil
//}

//func (s *Service) HandleDatasetFolderStatus(c *gin.Context) {
//	fmt.Println("You got the files-deployment search dataset folder status provider~!")
//	token := DatasetFolderStatusProviderRequest{
//		Token: c.GetHeader("X-Access-Token"),
//	}
//	if err := c.ShouldBindJSON(&token); err != nil {
//		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse request body"})
//		return
//	}
//
//	fmt.Println("search dataset folder status")
//	fmt.Println(token)
//	fmt.Println(token.Data)
//
//	req := c.Request
//	// 解析表单数据
//	if err := req.ParseForm(); err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse form data"})
//		return
//	}
//
//	var datasetNames = []string{}
//	var datasetIDs = []string{}
//	if len(token.Data.DatasetIDs) > 0 {
//		fmt.Println(token.Data.DatasetIDs)
//		datasetIDs = token.Data.DatasetIDs
//	}
//	if len(token.Data.DatasetNames) > 0 {
//		fmt.Println(token.Data.DatasetNames)
//		datasetNames = token.Data.DatasetNames
//	}
//	clean := token.Data.Clean
//	fmt.Println("clean=", clean)
//
//	var all int = 0
//	var newDatasetIDs = []string{}
//	var err error
//	if clean == 0 && (len(datasetNames) > 0 || len(datasetIDs) > 0) {
//		newDatasetIDs, err = processDatasetIDs(datasetIDs, datasetNames)
//		if err != nil {
//			c.JSON(http.StatusInternalServerError, gin.H{"error": "Process datasetIds and datasetNames failed"})
//			return
//		}
//	} else {
//		all = 1
//	}
//	fmt.Println("newDatasetIDs:", newDatasetIDs)
//
//	response, err := GetDatasetFolderStatus(newDatasetIDs, all, clean)
//	c.Header("Content-Type", "application/json")
//	defer func() {
//		if err == nil {
//			c.JSON(http.StatusOK, response)
//		} else {
//			c.JSON(http.StatusInternalServerError, "Internel server error")
//		}
//	}()
//}

//type DatasetFolderPathsProviderRequest struct {
//	Op       string                     `json:"op"`
//	DataType string                     `json:"datatype"`
//	Version  string                     `json:"version"`
//	Group    string                     `json:"group"`
//	Token    string                     `json:"token"`
//	Data     *DatasetFolderPathsRequest `json:"data"`
//}
//
//type DatasetFolderPathsRequest struct {
//	DatasetID      string   `json:"dataset_id"`
//	DatasetName    string   `json:"dataset_name"`
//	Paths          []string `json:"paths"`
//	CreateOrDelete int      `json:"create_or_delete"`
//}

//func GetDatasetIDsAndCreateNewDataset(datasetID, datasetName string, createOrDelete int) ([]string, error) {
//	if createOrDelete == 1 && datasetName == "" {
//		return nil, e.New("Only can create base a name")
//	}
//
//	if datasetName != "" {
//		resp, err := CallDifyGatewayBaseProvider(1, nil) //http.Get(url)
//		if err != nil {
//			return nil, fmt.Errorf("请求失败：%s\n", err.Error())
//		}
//
//		var datasetResponse struct {
//			Data struct {
//				Data []struct {
//					ID   string `json:"id"`
//					Name string `json:"name"`
//				} `json:"data"`
//			} `json:"data"`
//		}
//
//		err = json.NewDecoder(bytes.NewReader(resp)).Decode(&datasetResponse)
//		if err != nil {
//			return nil, fmt.Errorf("解析响应失败：%s\n", err.Error())
//		}
//
//		var datasetIDs []string
//
//		for _, dataItem := range datasetResponse.Data.Data {
//			if dataItem.Name == datasetName {
//				datasetIDs = append(datasetIDs, dataItem.ID)
//			}
//		}
//
//		if len(datasetIDs) > 0 {
//			if createOrDelete == 1 {
//				return datasetIDs, e.New("Dataset with this name already exists. Can't create a new one.")
//			}
//			return datasetIDs, nil
//		}
//
//		// if not found any, create a new dataset if createOrDelete == 1
//		if createOrDelete == 1 {
//			postData := make(map[string]interface{})
//			postData["name"] = datasetName
//
//			resp, err = CallDifyGatewayBaseProvider(2, postData)
//			if err != nil {
//				return nil, fmt.Errorf("Dataset Create Failed：%s\n", err.Error())
//			}
//
//			var createDatasetResponse struct {
//				Data struct {
//					ID string `json:"id"`
//				} `json:"data"`
//			}
//
//			err = json.Unmarshal(resp, &createDatasetResponse)
//			if err != nil {
//				return nil, fmt.Errorf("Parse dataset create response failed：%s\n", err.Error())
//			}
//
//			return []string{createDatasetResponse.Data.ID}, nil
//		}
//		return []string{}, nil
//	}
//
//	if datasetID != "" {
//		return []string{datasetID}, nil
//	}
//
//	return nil, fmt.Errorf("both datasetID and datasetName are empty")
//}

//func (s *Service) HandleDatasetFolderPaths(c *gin.Context) {
//	fmt.Println("You got the files-deployment dataset folder paths provider~!")
//	token := DatasetFolderPathsProviderRequest{
//		Token: c.GetHeader("X-Access-Token"),
//	}
//	if err := c.ShouldBindJSON(&token); err != nil {
//		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse request body"})
//		return
//	}
//
//	fmt.Println("dataset folder paths")
//	fmt.Println(token)
//	fmt.Println(token.Data)
//
//	req := c.Request
//	// 解析表单数据
//	if err := req.ParseForm(); err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse form data"})
//		return
//	}
//
//	// 添加paths字段
//	var datasetID = ""
//	var datasetName = ""
//	var paths = []string{}
//	var createOrDelete = 0
//	if token.Data.DatasetID != "" {
//		fmt.Println(token.Data.DatasetID)
//		datasetID = token.Data.DatasetID
//	}
//	if token.Data.DatasetName != "" {
//		fmt.Println(token.Data.DatasetName)
//		datasetName = token.Data.DatasetName
//	}
//	if len(token.Data.Paths) > 0 {
//		fmt.Println(token.Data.Paths)
//		paths = token.Data.Paths
//	} else {
//		//fmt.Println(token.Data.Query)
//		//paths = token.Data.Query
//	}
//	fmt.Println(token.Data.CreateOrDelete)
//	createOrDelete = token.Data.CreateOrDelete
//
//	fmt.Println(datasetID)
//	fmt.Println(datasetName)
//	fmt.Println(paths)
//
//	datasetIDs, err := GetDatasetIDsAndCreateNewDataset(datasetID, datasetName, createOrDelete)
//	if err != nil {
//		if err.Error() == "Dataset with this name already exists. Can't create a new one." {
//			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
//		} else if err.Error() == "Only can create base a name" {
//			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
//		} else {
//			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to identify dataset IDs!"})
//		}
//		return
//	}
//	fmt.Println("datasetIDs:", datasetIDs)
//
//	response := make(map[string]map[string]string)
//	for _, updateID := range datasetIDs {
//		UpdateDatasetFolderPaths(updateID, paths)
//		response[updateID] = make(map[string]string)
//		response[updateID]["datasetID"] = updateID
//		response[updateID]["paths"] = GetRedisPathsForDataset(updateID)
//		response[updateID]["status"] = "running"
//	}
//	c.Header("Content-Type", "application/json")
//	defer func() {
//		c.JSON(http.StatusOK, response)
//	}()
//
//	// 在原函数返回前调用新函数
//	s.SendUpdateDatasetFolderPathsRequest(false)
//	if createOrDelete == -1 {
//		for _, updateID := range datasetIDs {
//			deleteData := make(map[string]interface{})
//			deleteData["datasetID"] = updateID
//			UpdateDatasetFolderPaths(updateID, []string{})
//			deleteResp, err := CallDifyGatewayBaseProvider(5, deleteData)
//			if err != nil {
//				fmt.Println("Dataset Delete Failed: %s\n", err.Error())
//				return
//			}
//			fmt.Println(deleteResp)
//			response[updateID]["paths"] = ""
//			response[updateID]["status"] = "deleted"
//			my_redis.RedisDelKey("DATASET_" + updateID)
//		}
//	}
//}

//func (s *Service) SendUpdateDatasetFolderPathsRequest(init bool) {
//	if init {
//		for {
//			datasetIDs, err := getRedisDatasetIDs()
//			if err != nil {
//				fmt.Println("get datasetIDs from Redis failed: %s", err.Error())
//				return
//			}
//			fmt.Println("Current Redis DatasetIDs:", datasetIDs)
//			//if len(datasetIDs) == 0 {
//			basicDatasetName := BflName + "'s Document"
//			resp, err := CallDifyGatewayBaseProvider(1, nil) //http.Get(url)
//			if err != nil {
//				fmt.Println("Get Basic Dataset Failed!")
//				return
//			}
//
//			var datasetResponse struct {
//				Data struct {
//					Data []struct {
//						ID   string `json:"id"`
//						Name string `json:"name"`
//					} `json:"data"`
//				} `json:"data"`
//			}
//
//			err = json.NewDecoder(bytes.NewReader(resp)).Decode(&datasetResponse)
//			if err != nil {
//				fmt.Println("Parse Dataset Response Failed!")
//				return
//			}
//
//			var basicDatasetIDs []string
//
//			for _, dataItem := range datasetResponse.Data.Data {
//				if dataItem.Name == basicDatasetName {
//					basicDatasetIDs = append(basicDatasetIDs, dataItem.ID)
//				}
//			}
//
//			if len(basicDatasetIDs) > 0 {
//				fmt.Println("Basic DatasetID: ", basicDatasetIDs)
//				for _, basicDatasetID := range basicDatasetIDs {
//					curValue := my_redis.RedisGet("DATASET_" + basicDatasetID)
//					var curDataset map[string]interface{}
//					err = json.Unmarshal([]byte(curValue), &curDataset)
//					if err != nil {
//						fmt.Errorf("Parse Redis to JSON failed: %s\n", err.Error())
//					}
//					if err != nil || curValue == "" || curDataset["paths"] == nil {
//						fmt.Println("Basic Dataset ", basicDatasetID, " need to be initialized")
//						UpdateDatasetFolderPaths(basicDatasetID, []string{"/data/Home/Documents"})
//					} else {
//						fmt.Println("Basic Dataset ", basicDatasetID, " is now with paths ", curDataset["paths"])
//					}
//				}
//				break
//			}
//		}
//		// fmt.Println("Redis no data，won't disturb dify gateway")
//		// return
//		//}
//	}
//
//	resp, err := CallDifyGatewayBaseProvider(4, nil)
//	if err != nil {
//		fmt.Println("Failed to send HTTP request:", err)
//		return
//	}
//	fmt.Println(resp)
//}

//func (s *Service) HandleDatasetFolderStatusTest(c *gin.Context) {
//	fmt.Println("You got the files-deployment search dataset folder status provider~!")
//
//	req := c.Request
//
//	body, err := ioutil.ReadAll(req.Body)
//	if err != nil {
//		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
//		return
//	}
//	reqBody := string(body)
//
//	type Request struct {
//		DatasetNames []string `json:"datasetNames"`
//		DatasetIDs   []string `json:"datasetIDs"`
//		Clean        int      `json:"clean"`
//	}
//	var requestData Request
//	err = json.Unmarshal([]byte(reqBody), &requestData)
//	if err != nil {
//		// 处理解析错误
//	}
//
//	datasetNames := requestData.DatasetNames
//	datasetIDs := requestData.DatasetIDs
//	clean := requestData.Clean
//
//	fmt.Println("dataset folder status")
//	fmt.Println("datasetNames:", datasetNames)
//	fmt.Println("datasetIDs:", datasetIDs)
//	fmt.Println("clean:", clean)
//
//	var all int = 0
//	var newDatasetIDs = []string{}
//	if clean == 0 && (len(datasetNames) > 0 || len(datasetIDs) > 0) {
//		newDatasetIDs, err = processDatasetIDs(datasetIDs, datasetNames)
//		if err != nil {
//			c.JSON(http.StatusInternalServerError, gin.H{"error": "Process datasetIds and datasetNames failed"})
//			return
//		}
//	} else {
//		all = 1
//	}
//	fmt.Println("newDatasetIDs:", newDatasetIDs)
//
//	response, err := GetDatasetFolderStatus(newDatasetIDs, all, clean)
//	fmt.Println("dataset folder status response:", response)
//	c.Header("Content-Type", "application/json")
//	defer func() {
//		if err == nil {
//			c.JSON(http.StatusOK, response)
//		} else {
//			fmt.Println(err)
//			c.JSON(http.StatusInternalServerError, "Internel server error")
//		}
//	}()
//}

//func GetRedisPathsForDataset(datasetID string) string {
//	redisKey := fmt.Sprintf("DATASET_%s", datasetID)
//	type RedisResponse struct {
//		Paths []string `json:"paths"`
//	}
//
//	// 从 Redis 获取的字符串表示的 JSON 数据
//	redisValue := my_redis.RedisGet(redisKey)
//
//	var redisResponse RedisResponse
//	err := json.Unmarshal([]byte(redisValue), &redisResponse)
//	if err != nil {
//		fmt.Println("Failed to decode JSON:", err)
//		return ""
//	}
//
//	// 提取 paths 字段
//	redisPaths := redisResponse.Paths
//
//	// 将 paths 转换为字符串
//	redisPathsStr := strings.Join(redisPaths, ",")
//	return redisPathsStr
//}

//func (s *Service) HandleDatasetFolderPathsTest(c *gin.Context) {
//	fmt.Println("You got the files-deployment dataset folder paths provider~!")
//
//	req := c.Request
//	fmt.Println(req)
//
//	body, err := ioutil.ReadAll(req.Body)
//	if err != nil {
//		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
//		return
//	}
//	reqBody := string(body)
//
//	type Request struct {
//		Paths          []string `json:"paths"`
//		DatasetName    string   `json:"dataset_name"`
//		DatasetID      string   `json:"dataset_id"`
//		CreateOrDelete int      `json:"create_or_delete"`
//	}
//	var requestData Request
//	err = json.Unmarshal([]byte(reqBody), &requestData)
//	if err != nil {
//		// 处理解析错误
//	}
//
//	paths := requestData.Paths
//	datasetName := requestData.DatasetName
//	datasetID := requestData.DatasetID
//	createOrDelete := requestData.CreateOrDelete
//
//	fmt.Println("dataset folder paths")
//	fmt.Println("datasetID:", datasetID)
//	fmt.Println("datasetName:", datasetName)
//	fmt.Println("paths:", paths, ", length:", len(paths))
//
//	datasetIDs, err := GetDatasetIDsAndCreateNewDataset(datasetID, datasetName, createOrDelete)
//	if err != nil {
//		if err.Error() == "Dataset with this name already exists. Can't create a new one." {
//			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
//		} else if err.Error() == "Only can create base a name" {
//			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
//		} else {
//			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to identify dataset IDs!"})
//		}
//		return
//	}
//	fmt.Println("datasetIDs:", datasetIDs)
//
//	response := make(map[string]map[string]string)
//	for _, updateID := range datasetIDs {
//		UpdateDatasetFolderPaths(updateID, paths)
//		response[updateID] = make(map[string]string)
//		response[updateID]["datasetID"] = updateID
//		response[updateID]["paths"] = GetRedisPathsForDataset(updateID)
//		response[updateID]["status"] = "running"
//	}
//
//	c.Header("Content-Type", "application/json")
//	defer func() {
//		c.JSON(http.StatusOK, response)
//	}()
//
//	// 在原函数返回前调用新函数
//	s.SendUpdateDatasetFolderPathsRequest(false)
//	if createOrDelete == -1 {
//		for _, updateID := range datasetIDs {
//			deleteData := make(map[string]interface{})
//			deleteData["datasetID"] = updateID
//			UpdateDatasetFolderPaths(updateID, []string{})
//			deleteResp, err := CallDifyGatewayBaseProvider(5, deleteData)
//			if err != nil {
//				fmt.Println("Dataset Delete Failed: %s\n", err.Error())
//				return
//			}
//			fmt.Println(deleteResp)
//			response[updateID]["paths"] = ""
//			response[updateID]["status"] = "deleted"
//			my_redis.RedisDelKey("DATASET_" + updateID)
//		}
//	}
//}
