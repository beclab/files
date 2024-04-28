package rpc

import (
	"bytes"
	"fmt"
	"github.com/filebrowser/filebrowser/v2/common"
	"github.com/filebrowser/filebrowser/v2/my_redis"
	"github.com/filebrowser/filebrowser/v2/parser"
	"io/fs"
	"io/ioutil"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"bytetrade.io/web3os/fs-lib/jfsnotify"

	//jfsnotify "github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
)

var watcher *jfsnotify.Watcher = nil

func WatchPath(addPaths []string, deletePaths []string) {
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
				log.Info().Msgf("push indexer task delete %s", path)
				res, err := RpcServer.EsQueryByPath(FileIndex, path)
				if err != nil {
					return err
				}
				docs, err := EsGetFileQueryResult(res)
				if err != nil {
					return err
				}
				for _, doc := range docs {
					_, err = RpcServer.EsDelete(doc.DocId, FileIndex)
					if err != nil {
						log.Error().Msgf("zinc delete error %s", err.Error())
					}
					log.Debug().Msgf("delete doc id %s path %s", doc.DocId, path)
				}
				//err = updateOrInputDoc(path)
				if err != nil {
					log.Error().Msgf("udpate or input doc err %v", err)
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

	for _, path := range addPaths {
		err = filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				fmt.Println("Adding Path: ", info.Name())
				err = watcher.Add(path)
				if err != nil {
					fmt.Println("watcher add error:", err)
					return err
				}
			} else {
				err = updateOrInputDoc(path)
				if err != nil {
					log.Error().Msgf("udpate or input doc err %v", err)
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
	if e.Has(jfsnotify.Remove) || e.Has(jfsnotify.Rename) {
		log.Info().Msgf("push indexer task delete %s", e.Name)
		res, err := RpcServer.EsQueryByPath(FileIndex, e.Name)
		if err != nil {
			return err
		}
		docs, err := EsGetFileQueryResult(res)
		if err != nil {
			return err
		}
		for _, doc := range docs {
			_, err = RpcServer.EsDelete(doc.DocId, FileIndex)
			if err != nil {
				log.Error().Msgf("zinc delete error %s", err.Error())
			}
			log.Debug().Msgf("delete doc id %s path %s", doc.DocId, e.Name)
		}
		//return nil
	}

	if e.Has(jfsnotify.Create) { // || e.Has(jfsnotify.Write) || e.Has(jfsnotify.Chmod) {
		err := filepath.Walk(e.Name, func(docPath string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				//add dir to watch list
				err = watcher.Add(docPath)
				if err != nil {
					log.Error().Msgf("watcher add error:%v", err)
				}
			} else {
				//input zinc file
				err = updateOrInputDoc(docPath)
				if err != nil {
					log.Error().Msgf("update or input doc error %v", err)
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
		return updateOrInputDoc(e.Name)
	}
	return nil
}

func updateOrInputDoc(filepath string) error {
	log.Debug().Msg("try update or input" + filepath)
	res, err := RpcServer.EsQueryByPath(FileIndex, filepath)
	if err != nil {
		return err
	}
	docs, err := EsGetFileQueryResult(res)
	if err != nil {
		return err
	}
	// path exist update doc
	if len(docs) > 0 {
		log.Debug().Msgf("has doc %v", docs[0].Where)
		//delete redundant docs
		if len(docs) > 1 {
			for _, doc := range docs[1:] {
				log.Debug().Msgf("delete redundant docid %s path %s", doc.DocId, doc.Where)
				_, err := RpcServer.EsDelete(doc.DocId, FileIndex)
				if err != nil {
					log.Error().Msgf("zinc delete error %v", err)
				}
			}
		}
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
		if newMd5 != docs[0].Md5 {
			//doc changed
			fileType := parser.GetTypeFromName(filepath)
			if _, ok := parser.ParseAble[fileType]; ok {
				log.Info().Msgf("push indexer task insert %s", filepath)
				content, err := parser.ParseDoc(bytes.NewReader(b), filepath)
				if err != nil {
					return err
				}
				log.Debug().Msgf("update content from old doc id %s path %s", docs[0].DocId, filepath)
				_, err = RpcServer.EsUpdateFileContentFromOldDoc(FileIndex, content, newMd5, docs[0])
				return err
			}
			log.Debug().Msgf("doc format not parsable %s", filepath)
			return nil
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
	md5 := common.Md5File(bytes.NewReader(b))
	fileType := parser.GetTypeFromName(filepath)
	content := ""
	if _, ok := parser.ParseAble[fileType]; ok {
		log.Info().Msgf("push indexer task insert %s", filepath)
		content, err = parser.ParseDoc(bytes.NewBuffer(b), filepath)
		if err != nil {
			return err
		}
	}
	filename := path.Base(filepath)
	size := 0
	fileInfo, err := os.Stat(filepath)
	if err == nil {
		size = int(fileInfo.Size())
	}
	doc := map[string]interface{}{
		"name":        filename,
		"where":       filepath,
		"md5":         md5,
		"content":     content,
		"size":        size,
		"created":     time.Now().Unix(),
		"updated":     time.Now().Unix(),
		"format_name": FormatFilename(filename),
	}
	id, err := RpcServer.EsInput(FileIndex, doc)
	log.Debug().Msgf("zinc input doc id %s path %s", id, filepath)
	return err
}

func printTime(s string, args ...interface{}) {
	log.Info().Msgf(time.Now().Format("15:04:05.0000")+" "+s+"\n", args...)
}
