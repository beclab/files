package app

import (
	"context"
	"errors"
	"files/pkg/common"
	"files/pkg/diskcache"
	"files/pkg/drivers"
	"files/pkg/drivers/clouds/rclone"
	"files/pkg/drivers/sync/seahub/seaserv"
	"files/pkg/global"
	"files/pkg/hertz"
	"files/pkg/hertz/biz/dal"
	"files/pkg/img"
	"files/pkg/integration"
	"files/pkg/redisutils"
	"files/pkg/tasks"
	"files/pkg/watchers"
	"io"
	"net"
	"os"
	"strings"

	"github.com/spf13/afero"
	"gopkg.in/natefinch/lumberjack.v2"
	"k8s.io/klog/v2"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	v "github.com/spf13/viper"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	cfgFile string
)

const DefaultPort = "6317"

func init() {
	cobra.OnInitialize(initConfig)
	cobra.MousetrapHelpText = ""

	rootCmd.SetVersionTemplate("File Browser version {{printf \"%s\" .Version}}\n")

	flags := rootCmd.Flags()
	persistent := rootCmd.PersistentFlags()

	persistent.StringVarP(&cfgFile, "config", "c", "", "config file path")
	persistent.StringP("database", "d", "./filebrowser.db", "database path")
	flags.Bool("noauth", false, "use the noauth auther when using quick setup")
	flags.String("username", "admin", "username for the first user when using quick config")
	flags.String("password", "", "hashed password for the first user when using quick config (default \"admin\")")

	addServerFlags(flags)
}

func addServerFlags(flags *pflag.FlagSet) {
	flags.StringP("address", "a", "127.0.0.1", "address to listen on")
	flags.StringP("log", "l", "stdout", "log output")
	flags.StringP("port", "p", "8080", "port to listen on") // 8110
	flags.StringP("cert", "t", "", "tls certificate")
	flags.StringP("key", "k", "", "tls key")
	flags.StringP("root", "r", ".", "root to prepend to relative paths")
	flags.String("socket", "", "socket to listen to (cannot be used with address, port, cert nor key flags)")
	flags.Uint32("socket-perm", 0666, "unix socket file permissions")
	flags.StringP("baseurl", "b", "", "base url")
	flags.String("cache-dir", "", "file cache directory (disabled if empty)")
	flags.Int("img-processors", 4, "image processors count")
	flags.Bool("disable-thumbnails", false, "disable image thumbnails")
	flags.Bool("disable-preview-resize", false, "disable resize of image previews")
	flags.Bool("disable-exec", false, "disables Command Runner feature")
	flags.Bool("disable-type-detection-by-header", false, "disables type detection by reading file headers")
	flags.String("seafile-rpc-path", "", "RPC pipe path (Prefer environment variables FB_SEAFILE_RPC_PATH)")
}

var rootCmd = &cobra.Command{
	Use:   "filebrowser",
	Short: "A stylish web-based file browser",
	Long: `File Browser CLI lets you create the database to use with File Browser,
manage your users and all the configurations without acessing the
web interface.

If you've never run File Browser, you'll need to have a database for
it. Don't worry: you don't need to setup a separate database server.
We're using Bolt DB which is a single file database and all managed
by ourselves.

For this specific command, all the flags you have available (except
"config" for the configuration file), can be given either through
environment variables or configuration files.

If you don't set "config", it will look for a configuration file called
.filebrowser.{json, toml, yaml, yml} in the following directories:

- ./
- $HOME/
- /etc/filebrowser/

The precedence of the configuration values are as follows:

- flags
- environment variables
- configuration file
- database values
- defaults

The environment variables are prefixed by "FB_" followed by the option
name in caps. So to set "database" via an env variable, you should
set FB_DATABASE.

Also, if the database path doesn't exist, File Browser will enter into
the quick setup mode and a new database will be bootstraped and a new
user created with the credentials from options "username" and "password".`,
	Run: python(func(cmd *cobra.Command, args []string) {
		klog.Infoln(cfgFile)

		// Step1：Init postgres (including migration).
		// For share, search and other features in the future
		dal.Init()

		// Step2: Init redis
		// For watcher, preview, smb and other features in the future
		redisutils.InitRedis()
		// redisutils.InitFolderAndRedis() // todo

		// step3-0: clean buffer
		err := os.RemoveAll("/data/buffer")
		if err != nil {
			klog.Fatalf("clean buffer failed: %v", err)
		}

		// Step3-1: Build IMG service
		workersCount, err := cmd.Flags().GetInt("img-processors")
		checkErr(err)
		if workersCount < 1 {
			klog.Fatal("Image resize workers count could not be < 1")
		}
		img.New(workersCount) // init global image service

		// Step3-2: Build file cache
		diskcache.New(afero.NewOsFs(), common.CACHE_PREFIX)

		// step4: init commands
		rclone.NewCommandRclone()
		rclone.Command.InitServes()
		rclone.Command.StopJobs()

		// step5: build driver handler
		drivers.NewDriverHandler()

		// step6: init global
		config := ctrl.GetConfigOrDie()
		global.InitGlobalData(config)
		global.InitGlobalNodes(config)
		global.InitGlobalMounted()

		// step7: init seahub (for test now)
		seaserv.InitSeaRPC()

		// step8: integration
		integration.NewIntegrationManager()

		// step9: watcher
		var w = watchers.NewWatchers(context.Background(), config)
		watchers.AddToWatchers[corev1.Node](w, global.NodeGVR, global.GlobalNode.Handlerevent())
		watchers.AddToWatchers[appsv1.StatefulSet](w, appsv1.SchemeGroupVersion.WithResource("statefulsets"), global.GlobalData.HandlerEvent())
		watchers.AddToWatchers[integration.User](w, integration.UserGVR, integration.IntegrationManager().HandlerEvent())

		go w.Run(1)

		// step10: task manager
		tasks.NewTaskManager()
		tasks.TaskManager.GenerateKeepFile()

		// step11: Crontab
		// .	- CleanupOldFilesAndRedisEntries
		InitCrontabs()

		// step12: run hertz server
		hertz.HertzServer()

	}, pythonConfig{allowNoDB: true}),
}

func cleanupHandler(listener net.Listener, c chan os.Signal) {
	sig := <-c
	klog.Infof("Caught signal %s: shutting down.", sig)
	listener.Close()
	os.Exit(0)
}

func getRunParams(flags *pflag.FlagSet) *common.Server {
	server := common.NewDefaultServer()

	if val, set := getParamB(flags, "root"); set {
		server.Root = val
	}

	if val, set := getParamB(flags, "baseurl"); set {
		server.BaseURL = val
	}

	if val, set := getParamB(flags, "log"); set {
		server.Log = val
	}

	isSocketSet := false
	isAddrSet := false

	if val, set := getParamB(flags, "address"); set {
		server.Address = val
		isAddrSet = isAddrSet || set
	}

	if val, set := getParamB(flags, "port"); set {
		server.Port = val
		isAddrSet = isAddrSet || set
	}

	if val, set := getParamB(flags, "key"); set {
		server.TLSKey = val
		isAddrSet = isAddrSet || set
	}

	if val, set := getParamB(flags, "cert"); set {
		server.TLSCert = val
		isAddrSet = isAddrSet || set
	}

	if val, set := getParamB(flags, "socket"); set {
		server.Socket = val
		isSocketSet = isSocketSet || set
	}

	if isAddrSet && isSocketSet {
		checkErr(errors.New("--socket flag cannot be used with --address, --port, --key nor --cert"))
	}

	// Do not use saved Socket if address was manually set.
	if isAddrSet && server.Socket != "" {
		server.Socket = ""
	}

	_, disableThumbnails := getParamB(flags, "disable-thumbnails")
	server.EnableThumbnails = !disableThumbnails

	_, disablePreviewResize := getParamB(flags, "disable-preview-resize")
	server.ResizePreview = !disablePreviewResize

	_, disableTypeDetectionByHeader := getParamB(flags, "disable-type-detection-by-header")
	server.TypeDetectionByHeader = !disableTypeDetectionByHeader

	_, disableExec := getParamB(flags, "disable-exec")
	server.EnableExec = !disableExec

	if val, set := getParamB(flags, "rpc-path"); set {
		server.RPCPath = val
	}

	return server
}

// getParamB returns a parameter as a string and a boolean to tell if it is different from the default
//
// NOTE: we could simply bind the flags to viper and use IsSet.
// Although there is a bug on Viper that always returns true on IsSet
// if a flag is binded. Our alternative way is to manually check
// the flag and then the value from env/config/gotten by viper.
// https://github.com/spf13/viper/pull/331
func getParamB(flags *pflag.FlagSet, key string) (string, bool) {
	value, _ := flags.GetString(key)

	// If set on Flags, use it.
	if flags.Changed(key) {
		return value, true
	}

	// If set through viper (env, config), return it.
	if v.IsSet(key) {
		return v.GetString(key), true
	}

	// Otherwise use default value on flags.
	return value, false
}

func getParam(flags *pflag.FlagSet, key string) string {
	val, _ := getParamB(flags, key)
	return val
}

func setupLog(logMethod string) {
	klog.Infof("Klog set to %s", logMethod)

	switch logMethod {
	case "stdout":
		klog.SetOutput(io.Writer(os.Stdout))
	case "stderr":
		klog.SetOutput(io.Writer(os.Stderr))
	case "":
		klog.SetOutput(io.Discard)
	default:
		klog.SetOutput(&lumberjack.Logger{
			Filename:   logMethod,
			MaxSize:    100,
			MaxAge:     14,
			MaxBackups: 10,
		})
	}
}

func initConfig() {
	if cfgFile == "" {
		home, err := homedir.Dir()
		checkErr(err)
		v.AddConfigPath(".")
		v.AddConfigPath(home)
		v.AddConfigPath("/etc/filebrowser/")
		v.SetConfigName(".filebrowser")
	} else {
		v.SetConfigFile(cfgFile)
	}

	v.SetEnvPrefix("FB")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(v.ConfigParseError); ok {
			panic(err)
		}
		cfgFile = "No config file used"
		v.AutomaticEnv() // forced
	} else {
		cfgFile = "Using config file: " + v.ConfigFileUsed()
	}

	klog.Infof("=== ENVIRONMENT VARIABLES ===")
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "FB_") {
			klog.Infof("[ENV] %s", e)
		}
	}

	klog.Infof("=== VIPER SETTINGS ===")
	for _, key := range v.AllKeys() {
		klog.Infof("[VIPER] %s = %v", key, v.Get(key))
	}
}
