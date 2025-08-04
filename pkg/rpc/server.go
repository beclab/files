package rpc

import (
	"context"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

const DefaultPort = "6317"

var PhotosEnabled = os.Getenv("PHOTOS_ENABLED")

var PhotosPath = os.Getenv("PHOTOS_PATH") // "/Home/Pictures"

var once sync.Once

var RpcServer *Service

type Service struct {
	port          string
	context       context.Context
	CallbackGroup *gin.RouterGroup
	KubeConfig    *rest.Config
	k8sClient     *kubernetes.Clientset
}

func InitRpcService(ctx context.Context) {
	klog.Infoln("Init RPCSERVER!")

	port := os.Getenv("W_PORT")
	if port == "" {
		port = DefaultPort
	}

	once.Do(func() {
		config := ctrl.GetConfigOrDie()
		k8sClient := kubernetes.NewForConfigOrDie(config)

		RpcServer = &Service{
			port:       port,
			context:    ctx,
			KubeConfig: config,
			k8sClient:  k8sClient,
		}
		PVCs = NewPVCCache(RpcServer)
		BflPVCs = NewBflPVCCache(RpcServer)

		//load routes
		RpcServer.loadRoutes()

		//InitWatcher()
	})

	klog.Infoln("RPCSERVER to start!")
	rpcErr := RpcServer.Start()

	if rpcErr != nil {
		panic(rpcErr)
	}
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

func (c *Service) Start() error {
	address := "0.0.0.0:" + c.port
	go RpcEngine.Run(address)
	klog.Infof("start rpc on port:%s", c.port)
	return nil
}

func (c *Service) loadRoutes() error {
	//start gin
	gin.DefaultWriter = &LoggerMy{}
	RpcEngine = gin.Default()

	//cors middleware
	RpcEngine.SetTrustedProxies(nil)
	RpcEngine.GET("/files/healthcheck", func(c *gin.Context) {
		klog.Infoln("You really see me")
		c.String(http.StatusOK, "ok")
	})
	RpcEngine.POST("/api/photos/pre_check", c.preCheckHandler)

	c.CallbackGroup = RpcEngine.Group("/api/callback")
	klog.Info("init rpc server")
	return nil
}
