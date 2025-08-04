package rpc

//import (
//	"files/pkg/files"
//	"files/pkg/redisutils"
//	"io/fs"
//	"k8s.io/klog/v2"
//	"os"
//	"path/filepath"
//	"strings"
//	"sync"
//	"time"
//
//	"bytetrade.io/web3os/fs-lib/jfsnotify"
//)
//
//var WatcherEnabled = os.Getenv("WATCHER_ENABLED")
//
//var PathPrefix = os.Getenv("PATH_PREFIX") // "/Home"
//
//var RootPrefix = os.Getenv("ROOT_PREFIX") // "/data"
//
//var CacheRootPath = os.Getenv("CACHE_ROOT_PATH") // "/appcache"
//
//var AppDataPathPrefix = "/AppData"
//
//var ContentPath = os.Getenv("CONTENT_PATH") //	"/Home/Documents"
//
//var watcher *jfsnotify.Watcher = nil
//var WatchDirs []string     // focused dirs
//var BaseWatchDirs []string // like /data, /appcache
//
//func InitWatcher() {
//	watchDirStr := os.Getenv("WATCH_DIR")
//
//	if watchDirStr == "" {
//		WatchDirs = append(WatchDirs, "./Home/Documents")
//	} else {
//		WatchDirs = strings.Split(watchDirStr, ",")
//		for i, dir := range WatchDirs {
//			WatchDirs[i] = strings.TrimSpace(dir)
//		}
//	}
//	klog.Infoln("original watchDirs = ", WatchDirs)
//
//	if RootPrefix == "" {
//		RootPrefix = "/data"
//	}
//
//	if ContentPath == "" {
//		ContentPath = "/Home/Documents"
//	}
//
//	//watchDirs = rpc.ExpandPaths(watchDirs, RootPrefix)
//	klog.Infoln("focused watchDirs = ", WatchDirs)
//
//	BaseWatchDirs = []string{RootPrefix}
//	if CacheRootPath != "" {
//		BaseWatchDirs = append(BaseWatchDirs, CacheRootPath)
//	}
//
//	klog.Infoln("baseWatchDirs = ", BaseWatchDirs)
//
//	if WatcherEnabled == "True" {
//		go WatchPath(BaseWatchDirs, nil, WatchDirs)
//	}
//}
//
//func checkString(s string) bool {
//	hasBase := false
//	for _, v := range BaseWatchDirs {
//		if strings.HasPrefix(s, v) {
//			hasBase = true
//			if v != RootPrefix {
//				return true
//			}
//		}
//	}
//	if !hasBase {
//		return false
//	}
//
//	if strings.HasPrefix(s+"/", RootPrefix+files.ExternalPrefix) {
//		return false
//		//return true // change to watching external
//	}
//
//	slashCount := 0
//	for i, char := range s {
//		if char == '/' {
//			slashCount++
//			if slashCount == 3 {
//				remaining := s[i:]
//				for _, prefix := range WatchDirs {
//					if strings.HasPrefix(remaining, prefix) {
//						return true
//					}
//				}
//				return false
//			}
//		}
//	}
//	if slashCount == 1 || slashCount == 2 {
//		return true
//	}
//	return false
//}
//
//func WatchPath(addPaths []string, deletePaths []string, focusPaths []string) {
//	klog.Infoln("Begin watching path...")
//
//	// Create a new watcher.
//	var err error
//	if watcher == nil {
//		addPaths = dedupArray(addPaths, PathPrefix)
//		err = redisutils.RedisClient.Set("indexing_status", 0, time.Duration(0)).Err()
//		if err != nil {
//			klog.Errorln("Set key value failed: ", err)
//			return
//		}
//		err = redisutils.RedisClient.Set("indexing_error", false, time.Duration(0)).Err()
//		if err != nil {
//			klog.Errorln("Set key value failed: ", err)
//			return
//		}
//		err = redisutils.RedisClient.Set("paths", strings.Join(addPaths, ","), time.Duration(0)).Err()
//		if err != nil {
//			klog.Errorln("Set key value failed: ", err)
//			return
//		}
//
//		watcher, err = jfsnotify.NewWatcher("filesWatcher")
//		if err != nil {
//			subErr := redisutils.RedisClient.Set("indexing_error", true, time.Duration(0)).Err()
//			if subErr != nil {
//				klog.Errorln("Set key value failed: ", subErr)
//			}
//			panic(err)
//		}
//	}
//
//	currentTime := time.Now().Format(time.RFC3339)
//	err = redisutils.RedisClient.Set("last_update_time", currentTime, time.Duration(0)).Err()
//	if err != nil {
//		klog.Errorln("write to redis failed:", err)
//		subErr := redisutils.RedisClient.Set("indexing_error", true, time.Duration(0)).Err()
//		if subErr != nil {
//			klog.Errorln("Set key value failed: ", subErr)
//		}
//		return
//	}
//
//	err = redisutils.RedisClient.IncrBy("indexing_status", 1).Err()
//	if err != nil {
//		klog.Errorln("write to redis failed:", err)
//		return
//	}
//
//	for _, path := range deletePaths {
//		err = filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
//			if err != nil {
//				return err
//			}
//			if info.IsDir() {
//				err = watcher.Remove(path)
//				if err != nil {
//					klog.Errorln("watcher add error:", err)
//					return err
//				}
//			}
//			return nil
//		})
//		if err != nil {
//			subErr := redisutils.RedisClient.Set("indexing_error", true, time.Duration(0)).Err()
//			if subErr != nil {
//				klog.Errorln("Set key value failed: ", subErr)
//			}
//			subErr = redisutils.RedisClient.DecrBy("indexing_status", 1).Err()
//			if subErr != nil {
//				klog.Errorln("write to redis failed:", subErr)
//			}
//			panic(err)
//		}
//	}
//
//	for _, path := range addPaths {
//		err = filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
//			if err != nil {
//				return err
//			}
//			if info.IsDir() {
//				klog.Infoln("filepath.Base: ", filepath.Base(path))
//				// disable nats
//				if filepath.Base(path) == strings.Trim(files.ExternalPrefix, "/") {
//					//if filepath.Base(path) == strings.Trim(".uploadstemp", "/") {
//					//klog.Infoln("We will skip the folder:", path)
//					return filepath.SkipDir
//				}
//
//				if checkString(path) {
//					klog.Infoln("Adding Path: ", path)
//					err = watcher.Add(path)
//					if err != nil {
//						klog.Errorln("watcher add error:", err)
//						return err
//					}
//				}
//			} else {
//			}
//			return nil
//		})
//		if err != nil {
//			subErr := redisutils.RedisClient.Set("indexing_error", true, time.Duration(0)).Err()
//			if subErr != nil {
//				klog.Errorln("Set key value failed: ", subErr)
//			}
//			subErr = redisutils.RedisClient.DecrBy("indexing_status", 1).Err()
//			if subErr != nil {
//				klog.Errorln("write to redis failed:", subErr)
//			}
//			panic(err)
//		}
//	}
//	err = redisutils.RedisClient.Set("indexing_error", true, time.Duration(0)).Err()
//	if err != nil {
//		klog.Errorln("Set key value failed: ", err)
//		panic(err)
//	}
//	err = redisutils.RedisClient.DecrBy("indexing_status", 1).Err()
//	if err != nil {
//		klog.Errorln("write to redis failed:", err)
//		panic(err)
//	}
//
//	klog.Infoln("Finished watching path setup.")
//
//	// Start listening for events.
//	go dedupLoop(watcher)
//	klog.Infof("watching path %s", strings.Join(addPaths, ","))
//}
//
//func dedupLoop(w *jfsnotify.Watcher) {
//	var (
//		waitFor      = 1000 * time.Millisecond
//		mu           sync.Mutex
//		timers       = make(map[string]*time.Timer)
//		pendingEvent = make(map[string]jfsnotify.Event)
//		printEvent   = func(e jfsnotify.Event) {
//			klog.Infof("handle event %v %v", e.Op.String(), e.Name)
//		}
//	)
//
//	go func() {
//		for {
//			mu.Lock()
//			toProcess := make(map[string]*time.Timer)
//			for name, t := range timers {
//				toProcess[name] = t
//			}
//			mu.Unlock()
//
//			for name, t := range toProcess {
//				select {
//				case <-t.C:
//					mu.Lock()
//					if ev, ok := pendingEvent[name]; ok {
//						delete(pendingEvent, name)
//						delete(timers, name)
//						mu.Unlock()
//
//						printEvent(ev)
//						err := handleEvent(ev)
//						if err != nil {
//							klog.Errorf("handle watch file event error %s", err.Error())
//						}
//					} else {
//						mu.Unlock()
//					}
//				case <-time.After(waitFor):
//					continue
//				}
//			}
//			time.Sleep(100 * time.Millisecond)
//		}
//	}()
//
//	for {
//		select {
//		case err, ok := <-w.Errors:
//			if !ok {
//				return
//			}
//			printTime("ERROR: %s", err)
//
//		case e, ok := <-w.Events:
//			if !ok {
//				klog.Warning("watcher event channel closed")
//				return
//			}
//
//			if e.Has(jfsnotify.Chmod) {
//				continue
//			}
//
//			if strings.HasSuffix(filepath.Dir(e.Name), "/.uploadstemp") {
//				continue
//			}
//
//			mu.Lock()
//			pendingEvent[e.Name] = e
//			t, ok := timers[e.Name]
//			if !ok {
//				t = time.NewTimer(waitFor)
//				timers[e.Name] = t
//			} else {
//				t.Reset(waitFor)
//			}
//			mu.Unlock()
//		}
//	}
//}
//
//func handleEvent(e jfsnotify.Event) error {
//	if strings.HasSuffix(filepath.Dir(e.Name), "/.uploadstemp") {
//		//klog.Infoln("we won't deal with uploads temp dir")
//		return nil
//	}
//
//	var bflName string
//	var err error
//	if checkString(e.Name) {
//		bflName, err = PVCs.GetBfl(ExtractPvcFromURL(e.Name))
//		if err != nil {
//			klog.Info(err)
//		} else {
//			klog.Info(e.Name, ", bfl-name: ", bflName)
//		}
//	} else {
//		return nil
//	}
//
//	if e.Has(jfsnotify.Remove) || e.Has(jfsnotify.Rename) {
//		err = checkOrUpdatePhotosRedis(e.Name, "", 3)
//		if err != nil {
//			klog.Errorf("check or update photos redis err %v", err)
//		}
//		//next line must be commented for rename
//		//return nil
//	}
//
//	if e.Has(jfsnotify.Create) {
//		err = filepath.Walk(e.Name, func(docPath string, info fs.FileInfo, err error) error {
//			if err != nil {
//				return err
//			}
//			if info.IsDir() {
//				if checkString(docPath) {
//					klog.Infoln("Adding Path: ", docPath)
//					err = watcher.Add(docPath)
//					if err != nil {
//						klog.Errorf("watcher add error:%v", err)
//					}
//				}
//			} else {
//				if checkString(docPath) {
//					err = checkOrUpdatePhotosRedis(docPath, "", 2)
//					if err != nil {
//						klog.Errorf("check or update photos redis err %v", err)
//					}
//				}
//			}
//			return nil
//		})
//		if err != nil {
//			klog.Errorf("handle create file error %v", err)
//		}
//		return nil
//	}
//
//	if e.Has(jfsnotify.Write) {
//	}
//	return nil
//}
//
//func printTime(s string, args ...interface{}) {
//	klog.Infof(time.Now().Format("15:04:05.0000")+" "+s+"\n", args...)
//}
