package diskcache

import (
	"context"
	"errors"
	"files/pkg/files"
	"files/pkg/global"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/afero"
	"k8s.io/klog/v2"
)

var fileCache *FileCache

// var CacheDir = os.Getenv("FILE_CACHE_DIR") // "/data/file_cache"

type FileCache struct {
	fs afero.Fs

	// granular locks
	scopedLocks struct {
		sync.Mutex
		sync.Once
		locks map[string]sync.Locker
	}
}

func New(fs afero.Fs, root string) *FileCache {
	fileCache = &FileCache{
		fs: afero.NewBasePathFs(fs, root),
	}
	return fileCache
}

func GetFileCache() *FileCache {
	return fileCache
}

func (f *FileCache) Store(ctx context.Context, owner string, key string, tag string, value []byte) error {
	mu := f.getScopedLocks(owner + key)
	mu.Lock()
	defer mu.Unlock()

	prefixPath := f.formatPath(owner, tag)
	fileName := prefixPath + key
	klog.Infof("discache store, fileName: %s", fileName)
	// forced 1000
	if err := f.fs.MkdirAll(filepath.Dir(fileName), 0700); err != nil {
		return err
	}
	if err := files.Chown(f.fs, filepath.Dir(fileName), 1000, 1000); err != nil {
		klog.Errorf("can't chown directory %s to user %d: %s", filepath.Dir(fileName), 1000, err)
		return err
	}

	if err := afero.WriteFile(f.fs, fileName, value, 0700); err != nil {
		return err
	}

	return nil
}

func (f *FileCache) Load(ctx context.Context, owner string, key string, tag string) (value []byte, exist bool, err error) {
	prefixPath := f.formatPath(owner, tag)
	r, ok, err := f.open(prefixPath, key)
	if err != nil || !ok {
		return nil, ok, err
	}
	defer r.Close()

	value, err = io.ReadAll(r)
	if err != nil {
		return nil, false, err
	}
	return value, true, nil
}

func (f *FileCache) Delete(ctx context.Context, owner string, key string, tag string) error {
	mu := f.getScopedLocks(owner + key)
	mu.Lock()
	defer mu.Unlock()

	prefixPath := f.formatPath(owner, tag)
	if err := f.fs.Remove(prefixPath + key); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (f *FileCache) formatPath(owner string, tag string) string {
	var p = filepath.Join(global.GlobalData.GetPvcCache(owner), DefaultRootPath, tag)
	return p + "/"
}

func (f *FileCache) open(prefixPath string, key string) (afero.File, bool, error) {
	p := prefixPath + key
	file, err := f.fs.Open(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}

	return file, true, nil
}

// getScopedLocks pull lock from the map if found or create a new one
func (f *FileCache) getScopedLocks(key string) (lock sync.Locker) {
	f.scopedLocks.Do(func() { f.scopedLocks.locks = map[string]sync.Locker{} })

	f.scopedLocks.Lock()
	lock, ok := f.scopedLocks.locks[key]
	if !ok {
		lock = &sync.Mutex{}
		f.scopedLocks.locks[key] = lock
	}
	f.scopedLocks.Unlock()

	return lock
}
