package rpc

import (
	"files/pkg/files"
	"files/pkg/fileutils"
	"files/pkg/parser"
	"files/pkg/redisutils"
	"io/fs"
	"k8s.io/klog/v2"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"bytetrade.io/web3os/fs-lib/jfsnotify"
)

var WatcherEnabled = os.Getenv("WATCHER_ENABLED")

var PathPrefix = os.Getenv("PATH_PREFIX") // "/Home"

var RootPrefix = os.Getenv("ROOT_PREFIX") // "/data"

var CacheRootPath = os.Getenv("CACHE_ROOT_PATH") // "/appcache"

var AppDataPathPrefix = "/AppData"

var ContentPath = os.Getenv("CONTENT_PATH") //	"/Home/Documents"

var watcher *jfsnotify.Watcher = nil
var WatchDirs []string     // focused dirs
var BaseWatchDirs []string // like /data, /appcache

func InitWatcher() {
	watchDirStr := os.Getenv("WATCH_DIR")

	if watchDirStr == "" {
		WatchDirs = append(WatchDirs, "./Home/Documents")
	} else {
		WatchDirs = strings.Split(watchDirStr, ",")
		for i, dir := range WatchDirs {
			WatchDirs[i] = strings.TrimSpace(dir)
		}
	}
	klog.Infoln("original watchDirs = ", WatchDirs)

	if RootPrefix == "" {
		RootPrefix = "/data"
	}

	if ContentPath == "" {
		ContentPath = "/Home/Documents"
	}

	//watchDirs = rpc.ExpandPaths(watchDirs, RootPrefix)
	klog.Infoln("focused watchDirs = ", WatchDirs)

	BaseWatchDirs = []string{RootPrefix}
	if CacheRootPath != "" {
		BaseWatchDirs = append(BaseWatchDirs, CacheRootPath)
	}

	klog.Infoln("baseWatchDirs = ", BaseWatchDirs)

	if WatcherEnabled == "True" {
		go WatchPath(BaseWatchDirs, nil, WatchDirs)
	}
}

func checkString(s string) bool {
	hasBase := false
	for _, v := range BaseWatchDirs {
		if strings.HasPrefix(s, v) {
			hasBase = true
			if v != RootPrefix {
				return true
			}
		}
	}
	if !hasBase {
		//klog.Infoln("!hasBase")
		return false
	}

	if strings.HasPrefix(s+"/", RootPrefix+files.ExternalPrefix) {
		//klog.Infoln(s+"/", RootPrefix+files.ExternalPrefix)
		return false
		//return true // change to watching external
	}

	slashCount := 0
	for i, char := range s {
		if char == '/' {
			slashCount++
			if slashCount == 3 {
				remaining := s[i:]
				for _, prefix := range WatchDirs {
					if strings.HasPrefix(remaining, prefix) {
						return true
					}
				}
				return false
			}
		}
	}
	//klog.Infoln("slashCount=", slashCount)
	if slashCount == 1 || slashCount == 2 {
		return true
	}
	return false
}

func WatchPath(addPaths []string, deletePaths []string, focusPaths []string) {
	klog.Infoln("Begin watching path...")

	// Create a new watcher.
	var err error
	if watcher == nil {
		addPaths = dedupArray(addPaths, PathPrefix)
		err = redisutils.RedisClient.Set("indexing_status", 0, time.Duration(0)).Err()
		if err != nil {
			klog.Errorln("Set key value failed: ", err)
			return
		}
		err = redisutils.RedisClient.Set("indexing_error", false, time.Duration(0)).Err()
		if err != nil {
			klog.Errorln("Set key value failed: ", err)
			return
		}
		err = redisutils.RedisClient.Set("paths", strings.Join(addPaths, ","), time.Duration(0)).Err()
		if err != nil {
			klog.Errorln("Set key value failed: ", err)
			return
		}

		watcher, err = jfsnotify.NewWatcher("filesWatcher")
		if err != nil {
			subErr := redisutils.RedisClient.Set("indexing_error", true, time.Duration(0)).Err()
			if subErr != nil {
				klog.Errorln("Set key value failed: ", subErr)
			}
			panic(err)
		}

		// Start listening for events.
		go dedupLoop(watcher)
		klog.Infof("watching path %s", strings.Join(addPaths, ","))
	}

	currentTime := time.Now().Format(time.RFC3339)
	err = redisutils.RedisClient.Set("last_update_time", currentTime, time.Duration(0)).Err()
	if err != nil {
		klog.Errorln("write to redis failed:", err)
		subErr := redisutils.RedisClient.Set("indexing_error", true, time.Duration(0)).Err()
		if subErr != nil {
			klog.Errorln("Set key value failed: ", subErr)
		}
		return
	}

	err = redisutils.RedisClient.IncrBy("indexing_status", 1).Err()
	if err != nil {
		klog.Errorln("write to redis failed:", err)
		return
	}

	for _, path := range deletePaths {
		err = filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				err = watcher.Remove(path)
				if err != nil {
					klog.Errorln("watcher add error:", err)
					return err
				}
			}
			return nil
		})
		if err != nil {
			subErr := redisutils.RedisClient.Set("indexing_error", true, time.Duration(0)).Err()
			if subErr != nil {
				klog.Errorln("Set key value failed: ", subErr)
			}
			subErr = redisutils.RedisClient.DecrBy("indexing_status", 1).Err()
			if subErr != nil {
				klog.Errorln("write to redis failed:", subErr)
			}
			panic(err)
		}
	}

	for _, path := range addPaths {
		err = filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				klog.Infoln("filepath.Base: ", filepath.Base(path))
				// disable nats
				if filepath.Base(path) == strings.Trim(files.ExternalPrefix, "/") {
					//if filepath.Base(path) == strings.Trim(".uploadstemp", "/") {
					//klog.Infoln("We will skip the folder:", path)
					return filepath.SkipDir
				}

				if checkString(path) {
					klog.Infoln("Adding Path: ", path)
					err = watcher.Add(path)
					if err != nil {
						klog.Errorln("watcher add error:", err)
						return err
					}
				}
			} else {
				var search3 bool = true
				if strings.HasPrefix(path, RootPrefix+files.ExternalPrefix) {
					klog.Info(path, RootPrefix+files.ExternalPrefix)
					search3 = false
				}
				if search3 && checkString(path) {
					bflName, err := PVCs.GetBfl(ExtractPvcFromURL(path))
					if err != nil {
						klog.Info(err)
					} else {
						klog.Info(path, ", bfl-name: ", bflName)
					}
					err = updateOrInputDocSearch3(path, bflName)
					if err != nil {
						klog.Errorf("udpate or input doc err %v", err)
					}
				}
			}
			return nil
		})
		if err != nil {
			subErr := redisutils.RedisClient.Set("indexing_error", true, time.Duration(0)).Err()
			if subErr != nil {
				klog.Errorln("Set key value failed: ", subErr)
			}
			subErr = redisutils.RedisClient.DecrBy("indexing_status", 1).Err()
			if subErr != nil {
				klog.Errorln("write to redis failed:", subErr)
			}
			panic(err)
		}
	}
	err = redisutils.RedisClient.Set("indexing_error", true, time.Duration(0)).Err()
	if err != nil {
		klog.Errorln("Set key value failed: ", err)
		panic(err)
	}
	err = redisutils.RedisClient.DecrBy("indexing_status", 1).Err()
	if err != nil {
		klog.Errorln("write to redis failed:", err)
		panic(err)
	}

	klog.Infoln("Finished watching path setup.")
}

func dedupLoop(w *jfsnotify.Watcher) {
	var (
		waitFor      = 1000 * time.Millisecond
		mu           sync.Mutex
		timers       = make(map[string]*time.Timer)
		pendingEvent = make(map[string]jfsnotify.Event)
		printEvent   = func(e jfsnotify.Event) {
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
						err := handleEvent(ev)
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

			if e.Has(jfsnotify.Chmod) {
				continue
			}

			if strings.HasSuffix(filepath.Dir(e.Name), "/.uploadstemp") {
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

func handleEvent(e jfsnotify.Event) error {
	if strings.HasSuffix(filepath.Dir(e.Name), "/.uploadstemp") {
		//klog.Infoln("we won't deal with uploads temp dir")
		return nil
	}

	// temporarily disable search3 for external
	var search3 bool = true
	if strings.HasPrefix(e.Name+"/", RootPrefix+files.ExternalPrefix) {
		klog.Infoln(e.Name+"/", RootPrefix+files.ExternalPrefix)
		search3 = false
	}

	searchId := ""
	var bflName string
	var err error
	if checkString(e.Name) {
		bflName, err = PVCs.GetBfl(ExtractPvcFromURL(e.Name))
		if err != nil {
			klog.Info(err)
		} else {
			klog.Info(e.Name, ", bfl-name: ", bflName)
		}
		if search3 {
			searchId, _, err = getSerachIdOrCache(e.Name, bflName, false)
			if err != nil {
				klog.Info(err)
			} else {
				klog.Info(e.Name, ", searchId: ", searchId)
			}
		}
	} else {
		return nil
	}

	if e.Has(jfsnotify.Remove) || e.Has(jfsnotify.Rename) {
		//var msg string
		//if e.Has(jfsnotify.Remove) {
		//	msg = "Remove event: " + e.Name
		//} else if e.Has(jfsnotify.Rename) {
		//	msg = "Rename event: " + e.Name
		//}
		//nats.SendMessage(msg)

		//disable nats
		//klog.Infoln("Add Remove or Rename Event: ", e.Name)
		//nats.AddEventToQueue(e, true)

		klog.Infof("push indexer task delete %s", e.Name)

		if search3 && searchId != "" {
			_, err = deleteDocumentSearch3(searchId, bflName)
			if err != nil {
				return err
			}
		}
		err = checkOrUpdatePhotosRedis(e.Name, "", 3)
		if err != nil {
			klog.Errorf("check or update photos redis err %v", err)
		}
		//next line must be commented for rename
		//return nil
	}

	if e.Has(jfsnotify.Create) {
		//var msg string
		//msg = "Create event: " + e.Name
		//nats.SendMessage(msg)

		//disable nats
		//klog.Infoln("Add Create Event: ", e.Name)
		////nats.AddEventToQueue(e)
		//var fileInfo fs.FileInfo
		//fileInfo, err = os.Stat(e.Name)
		//if err != nil {
		//	klog.Errorf("Error stating file %s: %v\n", e.Name, err)
		//	return err
		//}
		//if fileInfo.IsDir() {
		//	nats.AddEventToQueue(e, true)
		//	klog.Infoln("Directory created: ", e.Name)
		//} else {
		//	nats.AddEventToQueue(e, false)
		//	klog.Infoln("File created: ", e.Name)
		//}

		err = filepath.Walk(e.Name, func(docPath string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if checkString(docPath) {
					klog.Infoln("Adding Path: ", docPath)
					err = watcher.Add(docPath)
					if err != nil {
						klog.Errorf("watcher add error:%v", err)
					}
				}
			} else {
				if checkString(docPath) {
					if search3 {
						err = updateOrInputDocSearch3(docPath, bflName)
						if err != nil {
							klog.Errorf("update or input doc error %v", err)
						}
					}
					err = checkOrUpdatePhotosRedis(docPath, "", 2)
					if err != nil {
						klog.Errorf("check or update photos redis err %v", err)
					}
				}
			}
			return nil
		})
		if err != nil {
			klog.Errorf("handle create file error %v", err)
		}
		return nil
	}

	if e.Has(jfsnotify.Write) {
		//disable nats
		//klog.Infoln("Add Write Event: ", e.Name)
		//nats.AddEventToQueue(e, false)

		if search3 && checkString(e.Name) {
			return updateOrInputDocSearch3(e.Name, bflName)
		}
	}
	return nil
}

func printTime(s string, args ...interface{}) {
	klog.Infof(time.Now().Format("15:04:05.0000")+" "+s+"\n", args...)
}

func checkPathPrefix(filepath, prefix string) bool {
	parts := strings.Split(filepath, "/")

	if len(parts) < 4 {
		return false
	}

	remainingPath := "/" + strings.Join(parts[3:], "/")

	if strings.HasPrefix(remainingPath, prefix) {
		return true
	}
	return false
}

func updateOrInputDocSearch3(filepath, bflName string) error {
	searchId, _, err := getSerachIdOrCache(filepath, bflName, true)
	if err != nil {
		return err
	}

	// path exist update doc
	if searchId != "" {
		size := 0
		fileInfo, err := os.Stat(filepath)
		if err == nil {
			size = int(fileInfo.Size())
		}
		//doc changed
		fileType := parser.GetTypeFromName(filepath)
		content := "-"
		if checkPathPrefix(filepath, ContentPath) {
			if _, ok := parser.ParseAble[fileType]; ok {
				klog.Infof("push indexer task insert %s", filepath)
				content, err = parser.ParseDoc(filepath)
				if err != nil {
					klog.Errorf("parse doc error %v", err)
					return err
				}
				klog.Infof("update content from old search id %s path %s", searchId, filepath)
			}
		}
		var newDoc map[string]interface{} = nil
		if content != "" {
			newDoc = map[string]interface{}{
				"content": content,
				"meta": map[string]interface{}{
					"size":    size,
					"updated": time.Now().Unix(),
				},
			}
		} else {
			newDoc = map[string]interface{}{
				"content": "-",
				"meta": map[string]interface{}{
					"size":    size,
					"updated": time.Now().Unix(),
				},
			}
		}
		_, err = putDocumentSearch3(searchId, newDoc, bflName)
		return err
	}

	klog.Infof("no history doc, add new")
	fileType := parser.GetTypeFromName(filepath)
	content := "-"
	if checkPathPrefix(filepath, ContentPath) {
		if _, ok := parser.ParseAble[fileType]; ok {
			klog.Infof("push indexer task insert %s", filepath)
			content, err = parser.ParseDoc(filepath)
			if err != nil {
				klog.Errorf("parse doc error %v", err)
				return err
			}
		}
	}
	filename := path.Base(filepath)
	size := 0
	fileInfo, err := os.Stat(filepath)
	if err == nil {
		size = int(fileInfo.Size())
	}
	ownerUID, err := fileutils.GetUID(nil, filepath)
	if err != nil {
		return err
	}
	var doc map[string]interface{} = nil
	if content != "" {
		doc = map[string]interface{}{
			"title":        filename,
			"content":      content,
			"owner_userid": strconv.Itoa(ownerUID),
			"resource_uri": filepath,
			"service":      "files",
			"meta": map[string]interface{}{
				"size":        size,
				"created":     time.Now().Unix(),
				"updated":     time.Now().Unix(),
				"format_name": FormatFilename(filename),
			},
		}
	} else {
		doc = map[string]interface{}{
			"title":        filename,
			"content":      "-",
			"owner_userid": strconv.Itoa(ownerUID),
			"resource_uri": filepath,
			"service":      "files",
			"meta": map[string]interface{}{
				"size":        size,
				"created":     time.Now().Unix(),
				"updated":     time.Now().Unix(),
				"format_name": FormatFilename(filename),
			},
		}
	}
	id, err := postDocumentSearch3(doc, bflName)
	klog.Infof("search3 input doc id %s path %s", id, filepath)
	return err
}
