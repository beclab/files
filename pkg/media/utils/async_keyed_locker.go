package utils

import (
	"context"
	"sync"
)

type AsyncKeyedLocker struct {
	locks sync.Map
	cfg   AsyncKeyedLockerConfig
}

type AsyncKeyedLockerConfig struct {
	PoolSize        int
	PoolInitialFill int
}

func NewAsyncKeyedLocker(cfg AsyncKeyedLockerConfig) *AsyncKeyedLocker {
	return &AsyncKeyedLocker{
		cfg: cfg,
	}
}

func (l *AsyncKeyedLocker) LockAsync(ctx context.Context, key string) (func(), error) {
	var mu sync.Mutex
	mu.Lock()

	_, loaded := l.locks.LoadOrStore(key, &mu)
	if loaded {
		// If the key already exists, wait for the lock to be available
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			mutex, _ := l.locks.Load(key)
			(*mutex.(*sync.Mutex)).Lock()
		}
	}

	return func() {
		mutex, _ := l.locks.Load(key)
		(*mutex.(*sync.Mutex)).Unlock()
		//		(*l.locks.Load(key).(*sync.Mutex)).Unlock()
	}, nil
}

func (l *AsyncKeyedLocker) Dispose() {
	// No-op, as sync.Map doesn't require explicit cleanup
}

/*
type TranscodeManager struct {
	transcodingLocks *AsyncKeyedLocker
}

func NewTranscodeManager() *TranscodeManager {
	return &TranscodeManager{
		transcodingLocks: NewAsyncKeyedLocker(AsyncKeyedLockerConfig{
			PoolSize:        20,
			PoolInitialFill: 1,
		}),
	}
}

func (m *TranscodeManager) LockAsync(ctx context.Context, outputPath string) (func(), error) {
	return m.transcodingLocks.LockAsync(ctx, outputPath)
}

func (m *TranscodeManager) Dispose() {
	m.transcodingLocks.Dispose()
}

func main() {
	// Example usage
	manager := NewTranscodeManager()
	defer manager.Dispose()

	unlock, err := manager.LockAsync(context.Background(), "resource1")
	if err != nil {
		// Handle error
	}
	defer unlock()
	// Do something with the locked resource
}
*/
