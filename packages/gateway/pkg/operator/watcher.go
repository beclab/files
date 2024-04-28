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

package operator

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type Watcher struct {
	client *kubernetes.Clientset
	ctx    context.Context
}

func NewWatcher(ctx context.Context, config *rest.Config) *Watcher {
	return &Watcher{
		ctx:    ctx,
		client: kubernetes.NewForConfigOrDie(config),
	}
}

func (w *Watcher) Start() {
	go func() {
		reconnect := true
		var (
			nodeWatcher watch.Interface
			err         error
		)
		for {
			if w.ctx.Err() != nil {
				klog.Error(w.ctx.Err())
				return
			}

			if reconnect {
				klog.Info("start to watch node events")
				nodeWatcher, err = w.client.CoreV1().Nodes().Watch(w.ctx, metav1.ListOptions{})
				if err != nil {
					klog.Error("watch node error, ", err)
					time.Sleep(time.Second)
					continue
				}
			}

			reconnect = false

			select {
			case <-w.ctx.Done():
				nodeWatcher.Stop()
				klog.Info("node watcher stopped")

			case event, OK := <-nodeWatcher.ResultChan():
				if !OK {
					klog.Error("node watcher broken")
					reconnect = true
					continue
				}

				// retry 60 times
				retry := 0
				for retry < 60 {
					if err = w.reconcile(&event); err != nil {
						klog.Error("reconcile got err, ", err, ", will retry after 2 seconds")
						time.Sleep(2 * time.Second)
						retry += 1
					} else {
						retry = 60
					}
				}

				if err != nil {
					klog.Error("reconcile event error, and exceed retry times")
				}
			}
		}
	}()
}
