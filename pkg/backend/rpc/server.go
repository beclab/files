package rpc

import (
	"context"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"

	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var WatcherEnabled = os.Getenv("WATCHER_ENABLED")

var PathPrefix = os.Getenv("PATH_PREFIX") // "/Home"

var RootPrefix = os.Getenv("ROOT_PREFIX") // "/data"

var CacheRootPath = os.Getenv("CACHE_ROOT_PATH") // "/appcache"

var ContentPath = os.Getenv("CONTENT_PATH") //	"/Home/Documents"

var PhotosEnabled = os.Getenv("PHOTOS_ENABLED")

var PhotosPath = os.Getenv("PHOTOS_PATH") // "/Home/Pictures"

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
		ctxTemp := context.WithValue(context.Background(), "Username", username)
		ctx := context.WithValue(ctxTemp, "Password", password)

		config := ctrl.GetConfigOrDie()
		k8sClient := kubernetes.NewForConfigOrDie(config)

		RpcServer = &Service{
			port:             port,
			zincUrl:          url,
			username:         username,
			password:         password,
			context:          ctx,
			maxPendingLength: maxPendingLength,
			KubeConfig:       config,
			k8sClient:        k8sClient,
		}
		PVCs = NewPVCCache(RpcServer)
		BflPVCs = NewBflPVCCache(RpcServer)

		//load routes
		RpcServer.loadRoutes()
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
	RpcEngine.POST("/api/photos/pre_check", c.preCheckHandler)

	c.CallbackGroup = RpcEngine.Group("/api/callback")
	log.Printf("init rpc server")
	return nil
}
