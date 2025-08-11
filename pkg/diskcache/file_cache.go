package diskcache

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"files/pkg/files"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/afero"
	"k8s.io/klog/v2"
)

var fileCache *FileCache

var CacheDir = os.Getenv("FILE_CACHE_DIR") // "/data/file_cache"

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

func (f *FileCache) Store(ctx context.Context, key string, value []byte) error {
	mu := f.getScopedLocks(key)
	mu.Lock()
	defer mu.Unlock()

	fileName := f.getFileName(key)
	klog.Infoln("key: ", key, " fileName: ", fileName, " filePath: ", filepath.Dir(fileName))
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

func (f *FileCache) Load(ctx context.Context, key string) (value []byte, exist bool, err error) {
	r, ok, err := f.open(key)
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

func (f *FileCache) Delete(ctx context.Context, key string) error {
	mu := f.getScopedLocks(key)
	mu.Lock()
	defer mu.Unlock()

	fileName := f.getFileName(key)
	if err := f.fs.Remove(fileName); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (f *FileCache) open(key string) (afero.File, bool, error) {
	fileName := f.getFileName(key)
	file, err := f.fs.Open(fileName)
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

func (f *FileCache) getFileName(key string) string {
	hasher := sha1.New()
	_, _ = hasher.Write([]byte(key))
	hash := hex.EncodeToString(hasher.Sum(nil))
	return hash
}
