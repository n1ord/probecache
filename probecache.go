package probecache

import (
	"fmt"
)

type IStorage interface {
	Set(key string, data []byte, ttl uint64) error
	Get(key string) ([]byte, error)
	GetWithTTL(key string) ([]byte, uint64, error)
	Del(key string) error
	Clear()

	GetSize() int
	PrintInfo()
}

const (
	offset64 = 14695981039346656037
	prime64  = 1099511628211
)

var (
	ErrMissing = fmt.Errorf("Entry not found in cache")
)
