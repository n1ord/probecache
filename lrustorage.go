package probecache

import (
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"time"
)

type LRUShard struct {
	sync.RWMutex
	data map[uint64][]byte

	maxSize       int
	critSize      int
	size          int
	maxCleanDepth int

	now        time.Time
	totalWorth float64

	// cleanDepth int
	// maxDepth   int
	// cleans     int
	// cleaned    int
}

func NewLRUShard(maxSize int, maxCritSize int, maxCleanDepth int, now time.Time) *LRUShard {
	if maxCritSize == 0 {
		maxCritSize = maxSize
	}
	s := &LRUShard{
		now:           now,
		maxSize:       maxSize,
		critSize:      maxCritSize,
		maxCleanDepth: maxCleanDepth,
	}
	s.data = make(map[uint64][]byte)
	return s
}

// Run in lock only
func (s *LRUShard) clean() {
	if s.maxSize <= 0 || s.size <= s.maxSize {
		return
	}
	// s.cleans++
	iter := s.maxCleanDepth
	threshold := s.totalWorth / float64(len(s.data))
	// i := 0
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
		// i++
	}
	// s.cleanDepth += i
	// if i > s.maxDepth {
	// 	s.maxDepth = i
	// }
}

func (s *LRUShard) GetWithTTL(key uint64) ([]byte, uint64, error) {
	s.Lock()
	data, ok := s.data[key]
	if ok {
		d, expire, worth := s.unwrapData(data)
		s.totalWorth -= worth
		if s.isExpired(expire) {
			s.size -= len(data)
			delete(s.data, key)
		} else {
			worth = s.setTs(data)
			s.data[key] = data
			s.totalWorth += worth
			s.Unlock()
			ttl := expire - uint64(time.Now().Unix())
			return d, ttl, nil
		}
	}
	s.Unlock()
	return nil, 0, ErrMissing
}

func (s *LRUShard) Get(key uint64) ([]byte, error) {
	d, _, err := s.GetWithTTL(key)
	return d, err
}

func (s *LRUShard) Set(key uint64, data []byte, ttl uint64) error {
	s.Lock()
	e, ok := s.data[key]
	worth := 0.0
	if ok {
		d, _, w := s.unwrapData(e)
		s.size -= len(d)
		worth = w
	} else {
		s.clean()
	}
	d := s.wrapData(data, ttl, worth)
	s.size += len(d)
	s.data[key] = d
	s.Unlock()
	return nil
}

func (s *LRUShard) Del(key uint64) error {
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

func (s *LRUShard) Clear() {
	s.data = make(map[uint64][]byte)
	// s.cleans = 0
	// s.cleaned = 0
	s.totalWorth = 0
	s.size = 0
	// s.cleanDepth = 0
	s.now = time.Now()
}

// ----------------------------------------------

func (s *LRUShard) setTs(d []byte) float64 {
	ts := time.Since(s.now).Seconds()
	bits := math.Float64bits(ts)
	binary.BigEndian.PutUint64(d[8:16], bits)
	return ts
}

func (s *LRUShard) wrapData(d []byte, ttl uint64, worth float64) []byte {
	expire := uint64(time.Now().Unix()) + ttl
	out := make([]byte, len(d)+8+8)
	copy(out[16:], d)
	binary.BigEndian.PutUint64(out[0:8], expire)
	binary.BigEndian.PutUint64(out[8:16], math.Float64bits(worth))
	return out
}

func (s *LRUShard) unwrapData(d []byte) ([]byte, uint64, float64) {
	expire := binary.BigEndian.Uint64(d[0:8])
	worthbits := binary.BigEndian.Uint64(d[8:16])
	worth := math.Float64frombits(worthbits)
	return d[16:], expire, worth
}

func (s *LRUShard) isExpired(ts uint64) bool {
	now := uint64(time.Now().Unix())
	return ts <= now
}

func (s *LRUShard) GetSize() int {
	s.RLock()
	size := s.size
	s.RUnlock()
	return size
}

func (s *LRUShard) GetLen() int {
	s.RLock()
	size := len(s.data)
	s.RUnlock()
	return size
}

func (s *LRUShard) GetTTs() float64 {
	s.RLock()
	ts := s.totalWorth
	s.RUnlock()
	return ts
}

// ================================================================================================

type LRUStorage struct {
	NumShards     int
	MaxMemSize    int
	MaxCritSize   int
	MaxCleanDepth int

	now       time.Time
	shards    []*LRUShard
	shardMask uint64
}

func NewLRUStorage(numShards int, maxSize int, maxCritSize int, maxCleanDepth int) (*LRUStorage, error) {
	maxShardSize := maxSize / numShards
	critShardSize := maxCritSize / numShards
	s := &LRUStorage{
		NumShards:     numShards,
		MaxMemSize:    maxSize,
		MaxCritSize:   maxCritSize,
		MaxCleanDepth: maxCleanDepth,
		now:           time.Now(),
	}
	s.shards = make([]*LRUShard, numShards)
	for i := 0; i < numShards; i++ {
		s.shards[i] = NewLRUShard(maxShardSize, critShardSize, maxCleanDepth, s.now)
	}
	s.shardMask = uint64(numShards)
	return s, nil
}

func (s *LRUStorage) getKey(key string) uint64 {
	var hash uint64 = offset64
	for i := 0; i < len(key); i++ {
		hash ^= uint64(key[i])
		hash *= prime64
	}
	return hash
}

func (s *LRUStorage) getShard(key uint64) *LRUShard {
	i := key % s.shardMask
	// fmt.Printf("%d <=> %d\n", key&s.shardMask, i)
	// return s.shards[key&s.shardMask]
	return s.shards[i]
}

func (s *LRUStorage) Get(key string) ([]byte, error) {
	h := s.getKey(key)
	shard := s.getShard(h)
	data, err := shard.Get(h)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *LRUStorage) GetWithTTL(key string) ([]byte, uint64, error) {
	h := s.getKey(key)
	shard := s.getShard(h)
	data, ttl, err := shard.GetWithTTL(h)
	if err != nil {
		return nil, 0, err
	}
	return data, ttl, nil
}

func (s *LRUStorage) Set(key string, data []byte, ttl uint64) error {
	h := s.getKey(key)
	shard := s.getShard(h)
	return shard.Set(h, data, ttl)
}

func (s *LRUStorage) Del(key string) error {
	h := s.getKey(key)
	shard := s.getShard(h)
	return shard.Del(h)
}

func (s *LRUStorage) GetSize() int {
	size := 0
	for _, shard := range s.shards {
		size += shard.GetSize()
	}
	return size
}

func (s *LRUStorage) Clear() {
	for _, shard := range s.shards {
		shard.Clear()
	}
}

func (s *LRUStorage) PrintInfo() {
	fmt.Printf("Cache size: %dkb / %dkb / %dkb\n", s.GetSize()/1024, s.MaxMemSize/1024, s.MaxCritSize/1024)
	// for i, shard := range s.shards {
	// 	depth := shard.cleanDepth / shard.cleans
	// 	maxDepth := shard.maxDepth
	// 	cleanEff := float32(shard.cleaned) / float32(shard.cleans)
	// 	fmt.Printf("Shard #%d size=%d / %d / %d, len=%d, cleans=%d, avg clean depth=%d, maxDepth=%d, clean eff=%f\n", i, shard.GetSize(), shard.maxSize, shard.critSize, shard.GetLen(), shard.cleans, depth, maxDepth, cleanEff)
	// }
}
