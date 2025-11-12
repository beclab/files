package main

import (
	"context"
	v1 "files/pkg/apis/sys.bytetrade.io/v1"
	"files/pkg/client"
	"files/pkg/global"
	"files/pkg/hertz/biz/dal/database"
	"files/pkg/models"
	"files/pkg/samba"
	"files/pkg/watchers"

	"k8s.io/klog/v2"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use: "samba-share-server",
	Run: func(cmd *cobra.Command, args []string) {
		database.Init()

		f, err := client.NewFactory()
		if err != nil {
			klog.Fatalf("new factory error: %v", err)
		}

		config, err := f.ClientConfig()
		if err != nil {
			klog.Fatalf("get client config error: %v", err)
		}

		global.InitGlobalData(config)
		global.InitGlobalNodes(config)
		global.InitGlobalMounted()

		samba.NewSambaManager(f)
		samba.SambaService.Start()

		var w = watchers.NewWatchers(context.Background(), config)
		watchers.AddToWatchers[v1.ShareSamba](w, samba.SambaGVR, samba.SambaService.HandlerEvent())
		watchers.AddToWatchers[models.User](w, models.UserGVR, samba.SambaService.UserHandlerEvent())

		w.Run(1)
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		klog.Fatal(err)
	}
}
