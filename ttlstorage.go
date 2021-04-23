package probecache

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

type TTLShard struct {
	sync.RWMutex
	data map[uint64][]byte
	size int
}

func NewTTLShard() *TTLShard {
	s := &TTLShard{}
	s.data = make(map[uint64][]byte)
	return s
}

func (s *TTLShard) clean() {
	s.Lock()
	for k, data := range s.data {
		d, expire := s.unwrapData(data)
		if s.isExpired(expire) {
			s.size -= len(d)
			delete(s.data, k)
		}
	}
	s.Unlock()
}

func (s *TTLShard) GetWithTTL(key uint64) ([]byte, uint64, error) {
	s.RLock()
	data, ok := s.data[key]
	s.RUnlock()
	if ok {
		d, expire := s.unwrapData(data)
		if s.isExpired(expire) {
			s.Del(key)
		} else {
			ttl := expire - uint64(time.Now().Unix())
			return d, ttl, nil
		}
	}
	return nil, 0, ErrMissing
}

func (s *TTLShard) Get(key uint64) ([]byte, error) {
	d, _, err := s.GetWithTTL(key)
	return d, err
}

func (s *TTLShard) Set(key uint64, data []byte, ttl uint64) error {
	s.Lock()
	d, exist := s.data[key]
	if exist {
		s.size -= len(d)
	}
	d = s.wrapData(data, ttl)
	s.data[key] = d
	s.size += len(d)
	s.Unlock()
	return nil
}

func (s *TTLShard) Del(key uint64) error {
	s.Lock()
	data, ok := s.data[key]
	if ok {
		d, _ := s.unwrapData(data)
		s.size -= len(d)
		delete(s.data, key)
	}
	s.Unlock()
	return nil
}

func (s *TTLShard) Clear() {
	s.data = make(map[uint64][]byte)
}

// ----------------------------------------------

func (s *TTLShard) wrapData(d []byte, ttl uint64) []byte {
	expire := uint64(time.Now().Unix()) + ttl
	out := make([]byte, len(d)+8)
	copy(out[8:], d)
	binary.BigEndian.PutUint64(out[0:8], expire)
	return out
}

func (s *TTLShard) unwrapData(d []byte) ([]byte, uint64) {
	ts := binary.BigEndian.Uint64(d[0:8])
	return d[8:], ts
}

func (s *TTLShard) isExpired(ts uint64) bool {
	now := uint64(time.Now().Unix())
	return ts <= now
}

func (s *TTLShard) GetSize() int {
	s.RLock()
	size := s.size
	s.RUnlock()
	return size
}

func (s *TTLShard) GetLen() int {
	s.RLock()
	size := len(s.data)
	s.RUnlock()
	return size
}

// ================================================================================================

type TTLStorage struct {
	NumShards     int
	MaxCleanDepth int
	CleanPeriod   time.Duration

	stopCh    chan struct{}
	shards    []*TTLShard
	shardMask uint64
}

func NewTTLStorage(numShards int, cleanPeriod time.Duration) (*TTLStorage, error) {
	s := &TTLStorage{
		NumShards:   numShards,
		CleanPeriod: cleanPeriod,
	}
	s.shards = make([]*TTLShard, numShards)
	for i := 0; i < numShards; i++ {
		s.shards[i] = NewTTLShard()
	}
	s.shardMask = uint64(numShards)
	s.stopCh = make(chan struct{})
	if s.CleanPeriod > 0 {
		s.runCleaning()
	}

	return s, nil
}

func (s *TTLStorage) runCleaning() {
	go func() {
		for {
			select {
			case <-s.stopCh:
				return
			default:
				time.Sleep(s.CleanPeriod)
				for _, shard := range s.shards {
					shard.clean()
				}
			}
		}
	}()
}

func (s *TTLStorage) Close() {
	if s.CleanPeriod > 0 {
		s.stopCh <- struct{}{}
	}
}

func (s *TTLStorage) getKey(key string) uint64 {
	var hash uint64 = offset64
	for i := 0; i < len(key); i++ {
		hash ^= uint64(key[i])
		hash *= prime64
	}
	return hash
}

func (s *TTLStorage) getShard(key uint64) *TTLShard {
	i := key % s.shardMask
	return s.shards[i]
}

func (s *TTLStorage) Get(key string) ([]byte, error) {
	h := s.getKey(key)
	shard := s.getShard(h)
	data, err := shard.Get(h)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *TTLStorage) GetWithTTL(key string) ([]byte, uint64, error) {
	h := s.getKey(key)
	shard := s.getShard(h)
	data, ttl, err := shard.GetWithTTL(h)
	if err != nil {
		return nil, 0, err
	}
	return data, ttl, nil
}

func (s *TTLStorage) Set(key string, data []byte, ttl uint64) error {
	h := s.getKey(key)
	shard := s.getShard(h)
	return shard.Set(h, data, ttl)
}

func (s *TTLStorage) Del(key string) error {
	h := s.getKey(key)
	shard := s.getShard(h)
	return shard.Del(h)
}

func (s *TTLStorage) GetSize() int {
	size := 0
	for _, shard := range s.shards {
		size += shard.GetSize()
	}
	return size
}

func (s *TTLStorage) Clear() {
	for _, shard := range s.shards {
		shard.Clear()
	}
}

func (s *TTLStorage) PrintInfo() {
	fmt.Printf("Cache info:\n")
	for i, shard := range s.shards {
		fmt.Printf("Shard #%d size=%d, len=%d\n", i, shard.GetSize(), shard.GetLen())
	}
}
