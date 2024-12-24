package rpc

import (
	"bytes"
	"fmt"
	"github.com/filebrowser/filebrowser/v2/common"
	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/my_redis"
	"github.com/filebrowser/filebrowser/v2/nats"
	"github.com/filebrowser/filebrowser/v2/parser"
	"io/fs"
	"io/ioutil"
	"k8s.io/klog/v2"
	"math"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"bytetrade.io/web3os/fs-lib/jfsnotify"

	//jfsnotify "github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
)

var watcher *jfsnotify.Watcher = nil
var WatchDirs []string     // focused dirs
var BaseWatchDirs []string // like /data, /appcache

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
		fmt.Println("!hasBase")
		return false
	}

	if strings.HasPrefix(s+"/", RootPrefix+files.ExternalPrefix) {
		fmt.Println(s+"/", RootPrefix+files.ExternalPrefix)
		return false
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
	fmt.Println("slashCount=", slashCount)
	if slashCount == 1 || slashCount == 2 {
		return true
	}
	return false
}

func WatchPath(addPaths []string, deletePaths []string, focusPaths []string) {
	fmt.Println("Begin watching path...")
	//sleepDuration := 20 * time.Minute
	//time.Sleep(sleepDuration)
	//fmt.Println("20 minutes passed, continue watching path...")

	// Create a new watcher.
	var err error
	if watcher == nil {
		addPaths = dedupArray(addPaths, PathPrefix)
		my_redis.RedisSet("indexing_status", 0, time.Duration(0))
		my_redis.RedisSet("indexing_error", false, time.Duration(0))
		my_redis.RedisSet("paths", strings.Join(addPaths, ","), time.Duration(0))

		watcher, err = jfsnotify.NewWatcher("filesWatcher")
		//watcher, err = jfsnotify.NewWatcher()
		if err != nil {
			my_redis.RedisSet("indexing_error", true, time.Duration(0))
			panic(err)
		}

		// Start listening for events.
		go dedupLoop(watcher)
		log.Info().Msgf("watching path %s", strings.Join(addPaths, ","))
	}

	// 写入当前时间到 Redis
	currentTime := time.Now().Format(time.RFC3339)
	my_redis.RedisSet("last_update_time", currentTime, time.Duration(0))
	if err != nil {
		fmt.Println("写入失败:", err)
		my_redis.RedisSet("indexing_error", true, time.Duration(0))
		return
	}

	my_redis.RedisAddInt("indexing_status", 1, time.Duration(0))
	for _, path := range deletePaths {
		err = filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				err = watcher.Remove(path)
				if err != nil {
					fmt.Println("watcher add error:", err)
					return err
				}
			} else {
				//log.Info().Msgf("push indexer task delete %s", path)
				//res, err := RpcServer.EsQueryByPath(FileIndex, path)
				//if err != nil {
				//	return err
				//}
				//docs, err := EsGetFileQueryResult(res)
				//if err != nil {
				//	return err
				//}
				//for _, doc := range docs {
				//	_, err = RpcServer.EsDelete(doc.DocId, FileIndex)
				//	if err != nil {
				//		log.Error().Msgf("zinc delete error %s", err.Error())
				//	}
				//	log.Debug().Msgf("delete doc id %s path %s", doc.DocId, path)
				//}
				////err = updateOrInputDoc(path)
				//if err != nil {
				//	log.Error().Msgf("udpate or input doc err %v", err)
				//}
			}
			return nil
		})
		if err != nil {
			my_redis.RedisSet("indexing_error", true, time.Duration(0))
			my_redis.RedisAddInt("indexing_status", -1, time.Duration(0))
			panic(err)
		}
	}

	for _, path := range addPaths {
		err = filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				//fmt.Println("filepath.Base: ", filepath.Base(path))
				if filepath.Base(path) == strings.Trim(files.ExternalPrefix, "/") {
					fmt.Println("We will skip the folder:", path)
					return filepath.SkipDir
				}

				fmt.Println("Try to Add Path: ", path)
				if checkString(path) {
					fmt.Println("Adding Path: ", path)
					err = watcher.Add(path)
					if err != nil {
						fmt.Println("watcher add error:", err)
						return err
					}
				} else {
					fmt.Println("Won't add path: ", path)
				}
			} else {
				if checkString(path) {
					bflName, err := PVCs.getBfl(ExtractPvcFromURL(path))
					if err != nil {
						klog.Info(err)
					} else {
						klog.Info(path, ", bfl-name: ", bflName)
					}
					//err = updateOrInputDoc(path)
					err = updateOrInputDocSearch3(path, bflName)
					if err != nil {
						log.Error().Msgf("udpate or input doc err %v", err)
					}
				}
			}
			return nil
		})
		if err != nil {
			my_redis.RedisSet("indexing_error", true, time.Duration(0))
			my_redis.RedisAddInt("indexing_status", -1, time.Duration(0))
			panic(err)
		}
	}
	my_redis.RedisSet("indexing_error", false, time.Duration(0))
	my_redis.RedisAddInt("indexing_status", -1, time.Duration(0))
}

func dedupLoop(w *jfsnotify.Watcher) {
	var (
		// Wait 1000ms for new events; each new event resets the timer.
		waitFor = 1000 * time.Millisecond

		// Keep track of the timers, as path → timer.
		mu           sync.Mutex
		timers       = make(map[string]*time.Timer)
		pendingEvent = make(map[string]jfsnotify.Event)

		// Callback we run.
		printEvent = func(e jfsnotify.Event) {
			log.Info().Msgf("handle event %v %v", e.Op.String(), e.Name)

			// Don't need to remove the timer if you don't have a lot of files.
			mu.Lock()
			delete(pendingEvent, e.Name)
			delete(timers, e.Name)
			mu.Unlock()
		}
	)

	for {
		select {
		// Read from Errors.
		case err, ok := <-w.Errors:
			if !ok { // Channel was closed (i.e. Watcher.Close() was called).
				return
			}
			printTime("ERROR: %s", err)
		// Read from Events.
		case e, ok := <-w.Events:
			if !ok { // Channel was closed (i.e. Watcher.Close() was called).
				log.Warn().Msg("watcher event channel closed")
				return
			}
			if e.Has(jfsnotify.Chmod) {
				continue
			}
			log.Debug().Msgf("pending event %v", e)
			// Get timer.
			mu.Lock()
			pendingEvent[e.Name] = e
			t, ok := timers[e.Name]
			mu.Unlock()

			// No timer yet, so create one.
			if !ok {
				t = time.AfterFunc(math.MaxInt64, func() {
					mu.Lock()
					ev := pendingEvent[e.Name]
					mu.Unlock()
					printEvent(ev)
					err := handleEvent(ev)
					if err != nil {
						log.Error().Msgf("handle watch file event error %s", err.Error())
					}
				})
				t.Stop()

				mu.Lock()
				timers[e.Name] = t
				mu.Unlock()
			}

			// Reset the timer for this path, so it will start from 100ms again.
			t.Reset(waitFor)
		}
	}
}

func handleEvent(e jfsnotify.Event) error {
	searchId := ""
	var bflName string
	var err error
	if checkString(e.Name) {
		bflName, err = PVCs.getBfl(ExtractPvcFromURL(e.Name))
		if err != nil {
			klog.Info(err)
		} else {
			klog.Info(e.Name, ", bfl-name: ", bflName)
		}
		searchId, _, err = getSerachIdOrCache(e.Name, bflName, false)
		if err != nil {
			klog.Info(err)
		} else {
			klog.Info(e.Name, ", searchId: ", searchId)
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

		fmt.Println("Add Remove or Rename Event: ", e.Name)
		nats.AddEventToQueue(e)

		log.Info().Msgf("push indexer task delete %s", e.Name)
		//res, err := RpcServer.EsQueryByPath(FileIndex, e.Name)
		//if err != nil {
		//	return err
		//}
		//docs, err := EsGetFileQueryResult(res)
		//if err != nil {
		//	return err
		//}
		//for _, doc := range docs {
		//	_, err = RpcServer.EsDelete(doc.DocId, FileIndex)
		//	if err != nil {
		//		log.Error().Msgf("zinc delete error %s", err.Error())
		//	}
		//	log.Debug().Msgf("delete doc id %s path %s", doc.DocId, e.Name)
		//}

		if searchId != "" {
			_, err = deleteDocumentSearch3(searchId, bflName)
			if err != nil {
				return err
			}
		}
		err = checkOrUpdatePhotosRedis(e.Name, "", 3)
		if err != nil {
			log.Error().Msgf("check or update photos redis err %v", err)
		}
		//next line must be commented for rename
		//return nil
	}

	if e.Has(jfsnotify.Create) { // || e.Has(jfsnotify.Write) || e.Has(jfsnotify.Chmod) {
		//var msg string
		//msg = "Create event: " + e.Name
		//nats.SendMessage(msg)

		fmt.Println("Add Create Event: ", e.Name)
		nats.AddEventToQueue(e)

		err = filepath.Walk(e.Name, func(docPath string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				//add dir to watch list
				fmt.Println("Try to Add Path: ", docPath)
				if checkString(docPath) {
					fmt.Println("Adding Path: ", docPath)
					err = watcher.Add(docPath)
					if err != nil {
						log.Error().Msgf("watcher add error:%v", err)
					}
				} else {
					fmt.Println("Won't add Path: ", docPath)
				}
			} else {
				//input zinc file
				//err = updateOrInputDoc(docPath)
				if checkString(docPath) {
					err = updateOrInputDocSearch3(docPath, bflName)
					if err != nil {
						log.Error().Msgf("update or input doc error %v", err)
					}
					err = checkOrUpdatePhotosRedis(docPath, "", 2)
					if err != nil {
						log.Error().Msgf("check or update photos redis err %v", err)
					}
				}
			}
			return nil
		})
		if err != nil {
			log.Error().Msgf("handle create file error %v", err)
		}
		return nil
	}

	if e.Has(jfsnotify.Write) { // || e.Has(notify.Chmod) {
		fmt.Println("Add Write Event: ", e.Name)
		nats.AddEventToQueue(e)

		if checkString(e.Name) {
			return updateOrInputDocSearch3(e.Name, bflName)
		}
		//return updateOrInputDoc(e.Name)
	}
	return nil
}

//func updateOrInputDoc(filepath string) error {
//	log.Debug().Msg("try update or input" + filepath)
//	res, err := RpcServer.EsQueryByPath(FileIndex, filepath)
//	if err != nil {
//		return err
//	}
//	docs, err := EsGetFileQueryResult(res)
//	if err != nil {
//		return err
//	}
//	// path exist update doc
//	if len(docs) > 0 {
//		log.Debug().Msgf("has doc %v", docs[0].Where)
//		//delete redundant docs
//		if len(docs) > 1 {
//			for _, doc := range docs[1:] {
//				log.Debug().Msgf("delete redundant docid %s path %s", doc.DocId, doc.Where)
//				_, err := RpcServer.EsDelete(doc.DocId, FileIndex)
//				if err != nil {
//					log.Error().Msgf("zinc delete error %v", err)
//				}
//			}
//		}
//		//update if doc changed
//		f, err := os.Open(filepath)
//		if err != nil {
//			return err
//		}
//		b, err := ioutil.ReadAll(f)
//		f.Close()
//		if err != nil {
//			return err
//		}
//		newMd5 := common.Md5File(bytes.NewReader(b))
//		if newMd5 != docs[0].Md5 {
//			//doc changed
//			fileType := parser.GetTypeFromName(filepath)
//			if _, ok := parser.ParseAble[fileType]; ok {
//				log.Info().Msgf("push indexer task insert %s", filepath)
//				content, err := parser.ParseDoc(bytes.NewReader(b), filepath)
//				if err != nil {
//					return err
//				}
//				log.Debug().Msgf("update content from old doc id %s path %s", docs[0].DocId, filepath)
//				_, err = RpcServer.EsUpdateFileContentFromOldDoc(FileIndex, content, newMd5, docs[0])
//				return err
//			}
//			log.Debug().Msgf("doc format not parsable %s", filepath)
//			return nil
//		}
//		log.Debug().Msgf("ignore file %s md5: %s ", filepath, newMd5)
//		return nil
//	}
//
//	log.Debug().Msgf("no history doc, add new")
//	//path not exist input doc
//	f, err := os.Open(filepath)
//	if err != nil {
//		return err
//	}
//	b, err := ioutil.ReadAll(f)
//	f.Close()
//	if err != nil {
//		return err
//	}
//	md5 := common.Md5File(bytes.NewReader(b))
//	fileType := parser.GetTypeFromName(filepath)
//	content := ""
//	if _, ok := parser.ParseAble[fileType]; ok {
//		log.Info().Msgf("push indexer task insert %s", filepath)
//		content, err = parser.ParseDoc(bytes.NewBuffer(b), filepath)
//		if err != nil {
//			return err
//		}
//	}
//	filename := path.Base(filepath)
//	size := 0
//	fileInfo, err := os.Stat(filepath)
//	if err == nil {
//		size = int(fileInfo.Size())
//	}
//	doc := map[string]interface{}{
//		"name":        filename,
//		"where":       filepath,
//		"md5":         md5,
//		"content":     content,
//		"size":        size,
//		"created":     time.Now().Unix(),
//		"updated":     time.Now().Unix(),
//		"format_name": FormatFilename(filename),
//	}
//	id, err := RpcServer.EsInput(FileIndex, doc)
//	log.Debug().Msgf("zinc input doc id %s path %s", id, filepath)
//	return err
//}

func printTime(s string, args ...interface{}) {
	log.Info().Msgf(time.Now().Format("15:04:05.0000")+" "+s+"\n", args...)
}

func getFileOwnerUID(filename string) (uint32, error) {
	fileInfo, err := os.Stat(filename)
	if err != nil {
		return 0, err
	}

	statT, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("unable to convert Sys() type to *syscall.Stat_t")
	}

	// 返回文件的UID
	return statT.Uid, nil
}

func checkPathPrefix(filepath, prefix string) bool {
	parts := strings.Split(filepath, "/")

	if len(parts) < 4 {
		return false
	}

	remainingPath := "/" + strings.Join(parts[3:], "/")
	//fmt.Println("remainingPath:", remainingPath)

	if strings.HasPrefix(remainingPath, prefix) {
		return true
	}
	return false
}

func updateOrInputDocSearch3(filepath, bflName string) error {
	log.Debug().Msg("try update or input" + filepath)
	searchId, md5, err := getSerachIdOrCache(filepath, bflName, true)
	if err != nil {
		return err
	}

	// path exist update doc
	if searchId != "" {
		//log.Debug().Msgf("has doc %v", docs[0].Where)
		////delete redundant docs
		//if len(docs) > 1 {
		//	for _, doc := range docs[1:] {
		//		log.Debug().Msgf("delete redundant docid %s path %s", doc.DocId, doc.Where)
		//		_, err := RpcServer.EsDelete(doc.DocId, FileIndex)
		//		if err != nil {
		//			log.Error().Msgf("zinc delete error %v", err)
		//		}
		//	}
		//}
		//update if doc changed
		f, err := os.Open(filepath)
		if err != nil {
			return err
		}
		b, err := ioutil.ReadAll(f)
		f.Close()
		if err != nil {
			return err
		}
		newMd5 := common.Md5File(bytes.NewReader(b))
		if newMd5 != md5 {
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
					log.Info().Msgf("push indexer task insert %s", filepath)
					content, err = parser.ParseDoc(bytes.NewReader(b), filepath)
					//if len(content) > 100 {
					//	log.Info().Msgf("parsed document content: %s", content[:100])
					//} else {
					//	log.Info().Msgf("parsed document content: %s", content)
					//}
					if err != nil {
						log.Error().Msgf("parse doc error %v", err)
						return err
					}
					log.Debug().Msgf("update content from old search id %s path %s", searchId, filepath)
				}
			}
			var newDoc map[string]interface{} = nil
			if content != "" {
				newDoc = map[string]interface{}{
					"content": content,
					"meta": map[string]interface{}{
						"md5":     newMd5,
						"size":    size,
						"updated": time.Now().Unix(),
					},
				}
			} else {
				newDoc = map[string]interface{}{
					"content": "-",
					"meta": map[string]interface{}{
						"md5":     newMd5,
						"size":    size,
						"updated": time.Now().Unix(),
					},
				}
			}
			_, err = putDocumentSearch3(searchId, newDoc, bflName)
			//_, err = RpcServer.EsUpdateFileContentFromOldDoc(FileIndex, content, newMd5, docs[0])
			return err
			//}
			//log.Debug().Msgf("doc format not parsable %s", filepath)
			//return nil
		}
		log.Debug().Msgf("ignore file %s md5: %s ", filepath, newMd5)
		return nil
	}

	log.Debug().Msgf("no history doc, add new")
	//path not exist input doc
	f, err := os.Open(filepath)
	if err != nil {
		return err
	}
	b, err := ioutil.ReadAll(f)
	f.Close()
	if err != nil {
		return err
	}
	newMd5 := common.Md5File(bytes.NewReader(b))
	fileType := parser.GetTypeFromName(filepath)
	content := "-"
	if checkPathPrefix(filepath, ContentPath) {
		if _, ok := parser.ParseAble[fileType]; ok {
			log.Info().Msgf("push indexer task insert %s", filepath)
			content, err = parser.ParseDoc(bytes.NewBuffer(b), filepath)
			//if len(content) > 100 {
			//	log.Info().Msgf("parsed document content: %s", content[:100])
			//} else {
			//	log.Info().Msgf("parsed document content: %s", content)
			//}
			if err != nil {
				log.Error().Msgf("parse doc error %v", err)
				return err
			}
		}
	}
	//} else {
	//	log.Debug().Msgf("doc format not parsable %s", filepath)
	//	return nil
	//}
	filename := path.Base(filepath)
	size := 0
	fileInfo, err := os.Stat(filepath)
	if err == nil {
		size = int(fileInfo.Size())
	}
	ownerUID, err := getFileOwnerUID(filepath)
	if err != nil {
		return err
	}
	var doc map[string]interface{} = nil
	if content != "" {
		doc = map[string]interface{}{
			"title":        filename,
			"content":      content,
			"owner_userid": strconv.Itoa(int(ownerUID)),
			"resource_uri": filepath,
			"service":      "files",
			"meta": map[string]interface{}{
				"md5":         newMd5,
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
			"owner_userid": strconv.Itoa(int(ownerUID)),
			"resource_uri": filepath,
			"service":      "files",
			"meta": map[string]interface{}{
				"md5":         newMd5,
				"size":        size,
				"created":     time.Now().Unix(),
				"updated":     time.Now().Unix(),
				"format_name": FormatFilename(filename),
			},
		}
	}
	id, err := postDocumentSearch3(doc, bflName)
	log.Debug().Msgf("search3 input doc id %s path %s", id, filepath)
	return err
}
