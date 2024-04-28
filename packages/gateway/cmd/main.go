// Copyright 2023 bytetrade
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"flag"

	"github.com/Above-Os/files/gateway/pkg/operator"
	"github.com/Above-Os/files/gateway/pkg/proxy"
	"github.com/Above-Os/files/gateway/pkg/signals"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
)

func main() {
	verbose := flag.Bool("v", false, "debug mode")
	addr := flag.String("addr", ":8080", "gateway listen address")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	_ = signals.SetupSignalHandler(ctx, cancel)

	config := ctrl.GetConfigOrDie()
	builder := &proxy.BackendProxyBuilder{
		Verbose:    *verbose,
		Addr:       *addr,
		MainCtx:    ctx,
		KubeConfig: config,
	}

	nodeWatcher := operator.NewWatcher(ctx, config)
	nodeWatcher.Start()

	proxy := builder.Build()

	go func() {
		<-ctx.Done()
		proxy.Shutdown()
	}()

	klog.Info("gateway start, listening on ", *addr)
	if err := proxy.Start(); err != nil {
		cancel()
	}
}
