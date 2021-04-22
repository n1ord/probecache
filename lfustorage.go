package probecache

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

type LFUShard struct {
	sync.RWMutex
	data map[uint64][]byte

	maxCleanDepth int
	maxSize       int
	critSize      int

	size       int
	totalWorth uint64

	// cleanDepth int
	// maxDepth   int
	// cleans     int
	// cleaned    int
}

func NewLFUShard(maxSize int, critSize int, maxCleanDepth int) *LFUShard {
	if critSize == 0 {
		critSize = maxSize
	}
	s := &LFUShard{
		maxSize:       maxSize,
		critSize:      critSize,
		maxCleanDepth: maxCleanDepth,
	}
	s.data = make(map[uint64][]byte)
	return s
}

// Run in lock only
func (s *LFUShard) clean() {
	if s.maxSize <= 0 || s.size <= s.maxSize {
		return
	}
	// s.cleans++
	iter := s.maxCleanDepth
	threshold := s.totalWorth / uint64(len(s.data))
	i := 0
	for k, data := range s.data {
		if s.size <= s.maxSize || iter == -2 || (iter <= 0 && s.size < s.critSize) {
			break
		}
		_, expire, worth := s.unwrapData(data)
		if worth <= threshold || s.isExpired(expire) || iter <= 0 {
			// s.cleaned++
			s.totalWorth -= worth
			s.size -= len(data)
			delete(s.data, k)
		}
		iter--
		i++
	}
	// s.cleanDepth += i
	// if i > s.maxDepth {
	// 	s.maxDepth = i
	// }
}

func (s *LFUShard) GetWithTTL(key uint64) ([]byte, uint64, error) {
	s.Lock()
	data, ok := s.data[key]
	if ok {
		d, expire, worth := s.unwrapData(data)
		if s.isExpired(expire) {
			s.totalWorth -= worth
			s.size -= len(data)
			delete(s.data, key)
		} else {
			s.incHit(data)
			s.data[key] = data
			s.totalWorth++
			s.Unlock()
			ttl := expire - uint64(time.Now().Unix())
			return d, ttl, nil
		}
	}
	s.Unlock()
	return nil, 0, ErrMissing
}

func (s *LFUShard) Get(key uint64) ([]byte, error) {
	d, _, err := s.GetWithTTL(key)
	return d, err
}

func (s *LFUShard) Set(key uint64, data []byte, ttl uint64) error {
	s.Lock()
	e, ok := s.data[key]
	worth := uint64(0)
	if ok {
		d, _, w := s.unwrapData(e)
		worth = w
		s.size -= len(d)
	} else {
		s.clean()
	}
	d := s.wrapData(data, ttl, worth)
	s.size += len(d)
	s.data[key] = d
	s.Unlock()
	return nil
}

func (s *LFUShard) Del(key uint64) error {
	s.Lock()
	data, ok := s.data[key]
	if ok {
		_, _, worth := s.unwrapData(data)
		delete(s.data, key)
		s.totalWorth -= worth
		s.size -= len(data)
	}
	s.Unlock()
	return nil
}

func (s *LFUShard) Clear() {
	s.data = make(map[uint64][]byte)
	// s.cleans = 0
	// s.cleaned = 0
	s.totalWorth = 0
	s.size = 0
	// s.cleanDepth = 0
}

// ----------------------------------------------

func (s *LFUShard) incHit(d []byte) {
	worth := binary.BigEndian.Uint64(d[8:16])
	worth++
	binary.BigEndian.PutUint64(d[8:16], worth)
}

func (s *LFUShard) wrapData(d []byte, ttl uint64, worth uint64) []byte {
	expire := uint64(time.Now().Unix()) + ttl
	out := make([]byte, len(d)+8+8)
	copy(out[16:], d)
	binary.BigEndian.PutUint64(out[0:8], expire)
	binary.BigEndian.PutUint64(out[8:16], worth)
	return out
}

func (s *LFUShard) unwrapData(d []byte) ([]byte, uint64, uint64) {
	ts := binary.BigEndian.Uint64(d[0:8])
	worth := binary.BigEndian.Uint64(d[8:16])
	return d[16:], ts, worth
}

func (s *LFUShard) isExpired(ts uint64) bool {
	now := uint64(time.Now().Unix())
	return ts <= now
}

func (s *LFUShard) GetSize() int {
	s.RLock()
	size := s.size
	s.RUnlock()
	return size
}

func (s *LFUShard) GetLen() int {
	s.RLock()
	size := len(s.data)
	s.RUnlock()
	return size
}

func (s *LFUShard) GetHits() uint64 {
	s.RLock()
	worth := s.totalWorth
	s.RUnlock()
	return worth
}

// ================================================================================================

type LFUStorage struct {
	NumShards     int
	MaxMemSize    int
	MaxCritSize   int
	MaxCleanDepth int

	shards    []*LFUShard
	shardMask uint64
}

func NewLFUStorage(numShards int, maxSize int, maxCritSize int, maxCleanDepth int) (*LFUStorage, error) {
	maxShardSize := maxSize / numShards
	critShardSize := maxCritSize / numShards
	s := &LFUStorage{
		NumShards:     numShards,
		MaxMemSize:    maxSize,
		MaxCritSize:   maxCritSize,
		MaxCleanDepth: maxCleanDepth,
	}
	s.shards = make([]*LFUShard, numShards)
	for i := 0; i < numShards; i++ {
		s.shards[i] = NewLFUShard(maxShardSize, critShardSize, maxCleanDepth)
	}
	s.shardMask = uint64(numShards)
	return s, nil
}

func (s *LFUStorage) getKey(key string) uint64 {
	var hash uint64 = offset64
	for i := 0; i < len(key); i++ {
		hash ^= uint64(key[i])
		hash *= prime64
	}
	return hash
}

func (s *LFUStorage) getShard(key uint64) *LFUShard {
	i := key % s.shardMask
	// fmt.Printf("%d <=> %d\n", key&s.shardMask, i)
	// return s.shards[key&s.shardMask]
	return s.shards[i]
}

func (s *LFUStorage) Get(key string) ([]byte, error) {
	h := s.getKey(key)
	shard := s.getShard(h)
	data, err := shard.Get(h)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *LFUStorage) GetWithTTL(key string) ([]byte, uint64, error) {
	h := s.getKey(key)
	shard := s.getShard(h)
	data, ttl, err := shard.GetWithTTL(h)
	if err != nil {
		return nil, 0, err
	}
	return data, ttl, nil
}

func (s *LFUStorage) Set(key string, data []byte, ttl uint64) error {
	h := s.getKey(key)
	shard := s.getShard(h)
	return shard.Set(h, data, ttl)
}

func (s *LFUStorage) Del(key string) error {
	h := s.getKey(key)
	shard := s.getShard(h)
	return shard.Del(h)
}

func (s *LFUStorage) GetSize() int {
	size := 0
	for _, shard := range s.shards {
		size += shard.GetSize()
	}
	return size
}

func (s *LFUStorage) Clear() {
	for _, shard := range s.shards {
		shard.Clear()
	}
}

func (s *LFUStorage) PrintInfo() {
	fmt.Printf("Cache size: %dkb / %dkb / %dkb\n", s.GetSize()/1024, s.MaxMemSize/1024, s.MaxCritSize/1024)
	// for i, shard := range s.shards {
	// 	depth := shard.cleanDepth / shard.cleans
	// 	maxDepth := shard.maxDepth
	// 	cleanEff := float32(shard.cleaned) / float32(shard.cleans)
	// 	fmt.Printf("Shard #%d size=%d / %d, len=%d, cleans=%d, avg clean depth=%d, maxDepth=%d, clean eff=%f\n", i, shard.GetSize(), shard.maxSize, shard.GetLen(), shard.cleans, depth, maxDepth, cleanEff)
	// }
}
