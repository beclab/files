package main

import (
	"context"
	v1 "files/pkg/apis/sys.bytetrade.io/v1"
	"files/pkg/client"
	"files/pkg/global"
	"files/pkg/hertz/biz/dal/database"
	"files/pkg/lifecycle"
	"files/pkg/models"
	"files/pkg/samba"
	"files/pkg/watchers"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"k8s.io/klog/v2"

	"github.com/spf13/cobra"
)

const (
	// sambaShutdownTimeout caps total teardown time after the first signal.
	sambaShutdownTimeout = 30 * time.Second
)

var rootCmd = &cobra.Command{
	Use: "samba-share-server",
	Run: func(cmd *cobra.Command, args []string) {
		if err := database.Init(); err != nil {
			klog.Fatalf("database.Init: %v", err)
		}

		f, err := client.NewFactory()
		if err != nil {
			klog.Fatalf("new factory error: %v", err)
		}

		config, err := f.ClientConfig()
		if err != nil {
			klog.Fatalf("get client config error: %v", err)
		}

		if err := global.InitGlobalData(config); err != nil {
			klog.Fatalf("init global data error: %v", err)
		}
		if err := global.InitGlobalNodes(config); err != nil {
			klog.Fatalf("init global nodes error: %v", err)
		}
		global.InitGlobalMounted()

		samba.NewSambaManager(f)
		samba.SambaService.Start()

		coord := lifecycle.New()
		coord.Add("postgres", 5*time.Second, func(context.Context) error {
			return database.Close()
		})
		coord.Add("external-fsnotify", 2*time.Second, func(context.Context) error {
			return global.ExternalWatcherClose()
		})
		coord.Add("samba-cleanup", 5*time.Second, func(ctx context.Context) error {
			return samba.SambaService.Stop(ctx)
		})

		watcherCtx, watcherCancel := context.WithCancel(context.Background())
		var w = watchers.NewWatchers(watcherCtx, config)
		if err := watchers.AddToWatchers[v1.ShareSamba](w, samba.SambaGVR, samba.SambaService.HandlerEvent()); err != nil {
			klog.Fatalf("register ShareSamba watcher: %v", err)
		}
		if err := watchers.AddToWatchers[models.User](w, models.UserGVR, samba.SambaService.UserHandlerEvent()); err != nil {
			klog.Fatalf("register User watcher: %v", err)
		}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			if runErr := w.Run(1); runErr != nil {
				klog.Errorf("samba watcher exited with error: %v", runErr)
			}
		}()
		coord.Add("k8s-watcher", 10*time.Second, func(ctx context.Context) error {
			watcherCancel()
			done := make(chan struct{})
			go func() { wg.Wait(); close(done) }()
			select {
			case <-done:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})

		// Signal handling: first SIGTERM/SIGINT triggers ordered shutdown
		// with a hard deadline; a second signal aborts immediately so an
		// operator can always force-exit a stuck binary.
		sigCh := make(chan os.Signal, 2)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		<-sigCh
		klog.Infof("samba: received shutdown signal, draining (timeout=%s)", sambaShutdownTimeout)
		go func() {
			<-sigCh
			klog.Warning("samba: second shutdown signal received, forcing exit")
			os.Exit(1)
		}()

		ctx, cancel := context.WithTimeout(context.Background(), sambaShutdownTimeout)
		defer cancel()
		coord.Run(ctx)
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		klog.Fatal(err)
	}
}
