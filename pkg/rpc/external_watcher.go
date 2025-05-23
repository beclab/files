package rpc

import (
	"bytetrade.io/web3os/fs-lib/jfsnotify"
	"files/pkg/drives"
	"files/pkg/files"
	"k8s.io/klog/v2"
	"sync"
	"time"
)

var externalWatcher *jfsnotify.Watcher = nil

func InitExternalWatcher() {
	var err error
	if externalWatcher == nil {
		externalWatcher, err = jfsnotify.NewWatcher("externalWatcher")
		if err != nil {
			klog.Fatalf("Failed to initialize watcher: %v", err)
			panic(err)
		}
		klog.Infoln("~~~Debug Log: externalWatcher created")

		// Start listening for events.
		go dedupExternalLoop(externalWatcher)
	}

	path := RootPrefix + files.ExternalPrefix
	klog.Infof("~~~Debug Log: Watching external files: %s", path)
	err = externalWatcher.Add(path)
	if err != nil {
		klog.Errorln("watcher add error:", err)
		panic(err)
	}
	klog.Infoln("~~~Debug Log: externalWatcher initialized with path:", path)
}

func dedupExternalLoop(w *jfsnotify.Watcher) {
	var (
		waitFor      = 1000 * time.Millisecond
		mu           sync.Mutex
		timers       = make(map[string]*time.Timer)
		pendingEvent = make(map[string]jfsnotify.Event)
		printEvent   = func(e jfsnotify.Event) {
			klog.Infof("handle event %v %v", e.Op.String(), e.Name)
		}
	)
	klog.Infof("~~~Debug Log: dedup watcher started with %v", w)

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

	klog.Infof("~~~Debug Log: dedup watcher started")
	for {
		klog.Infof("~~~Debug Log: event for")
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

			if e.Has(jfsnotify.Chmod) {
				continue
			}

			//if strings.HasSuffix(filepath.Dir(e.Name), "/.uploadstemp") {
			//	continue
			//}

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

func handleExternalEvent(e jfsnotify.Event) error {
	klog.Infof("~~~Debug Log: handle event %v %v", e.Op.String(), e.Name)
	//if strings.HasSuffix(filepath.Dir(e.Name), "/.uploadstemp") {
	//	//klog.Infoln("we won't deal with uploads temp dir")
	//	return nil
	//}

	if e.Has(jfsnotify.Remove) || e.Has(jfsnotify.Rename) {
		klog.Infof("external delete %s", e.Name)
		drives.GetMountedData(nil)
		//next line must be commented for rename
		//return nil
	}

	if e.Has(jfsnotify.Create) {
		klog.Infof("external create %s", e.Name)
		drives.GetMountedData(nil)
		return nil
	}

	if e.Has(jfsnotify.Write) {
		klog.Infof("external write %s", e.Name)
	}
	return nil
}
