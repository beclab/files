package rpc

import (
	"files/pkg/drives"
	"github.com/fsnotify/fsnotify"
	"k8s.io/klog/v2"
	"sync"
	"time"
)

var externalWatcher *fsnotify.Watcher = nil

func InitExternalWatcher() {
	var err error
	if externalWatcher == nil {
		externalWatcher, err = fsnotify.NewWatcher()
		if err != nil {
			klog.Fatalf("Failed to initialize watcher: %v", err)
			panic(err)
		}
	}

	path := "/data/External"
	err = externalWatcher.Add(path)
	if err != nil {
		klog.Errorln("watcher add error:", err)
		panic(err)
	}
	klog.Infof("watcher initialized at %s", path)

	// Start listening for events.
	go dedupExternalLoop(externalWatcher)
}

func dedupExternalLoop(w *fsnotify.Watcher) {
	var (
		waitFor      = 1000 * time.Millisecond
		mu           sync.Mutex
		timers       = make(map[string]*time.Timer)
		pendingEvent = make(map[string]fsnotify.Event)
		printEvent   = func(e fsnotify.Event) {
			klog.Infof("handle event %v %v", e.Op.String(), e.Name)
		}
	)

	go func() {
		for {
			mu.Lock()
			toProcess := make(map[string]*time.Timer)
			for name, t := range timers {
				toProcess[name] = t
			}
			mu.Unlock()

			for name, t := range toProcess {
				select {
				case <-t.C:
					mu.Lock()
					if ev, ok := pendingEvent[name]; ok {
						delete(pendingEvent, name)
						delete(timers, name)
						mu.Unlock()

						printEvent(ev)
						err := handleExternalEvent(ev)
						if err != nil {
							klog.Errorf("handle watch file event error %s", err.Error())
						}
					} else {
						mu.Unlock()
					}
				case <-time.After(waitFor):
					continue
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	for {
		select {
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			printTime("ERROR: %s", err)

		case e, ok := <-w.Events:
			if !ok {
				klog.Warning("watcher event channel closed")
				return
			}

			if e.Has(fsnotify.Chmod) {
				continue
			}

			mu.Lock()
			pendingEvent[e.Name] = e
			t, ok := timers[e.Name]
			if !ok {
				t = time.NewTimer(waitFor)
				timers[e.Name] = t
			} else {
				t.Reset(waitFor)
			}
			mu.Unlock()
		}
	}
}

func handleExternalEvent(e fsnotify.Event) error {
	if e.Has(fsnotify.Remove) || e.Has(fsnotify.Rename) {
		klog.Infof("external delete %s", e.Name)
		drives.GetMountedData(nil)
		//next line must be commented for rename
		//return nil
	}

	if e.Has(fsnotify.Create) {
		klog.Infof("external create %s", e.Name)
		drives.GetMountedData(nil)
		return nil
	}

	if e.Has(fsnotify.Write) {
		klog.Infof("external write %s", e.Name)
	}
	return nil
}
