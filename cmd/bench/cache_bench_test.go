package main

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/allegro/bigcache/v2"
	"github.com/coocood/freecache"
	"github.com/n1ord/probecache"
)

const maxEntrySize = 256
const numShards = 50

func BenchmarkMapSet(b *testing.B) {
	m := make(map[string][]byte, b.N)
	for i := 0; i < b.N; i++ {
		m[key(i)] = value()
	}
}

func BenchmarkConcurrentMapSet(b *testing.B) {
	var m sync.Map
	for i := 0; i < b.N; i++ {
		m.Store(key(i), value())
	}
}

func BenchmarkFreeCacheSet(b *testing.B) {
	cache := freecache.NewCache(b.N * maxEntrySize)
	for i := 0; i < b.N; i++ {
		cache.Set([]byte(key(i)), value(), 0)
	}
}

func BenchmarkBigCacheSet(b *testing.B) {
	cache := initBigCache(b.N)
	for i := 0; i < b.N; i++ {
		cache.Set(key(i), value())
	}
}

func BenchmarkProbeLRUSet(b *testing.B) {
	cache := initProbeLru(b.N)
	for i := 0; i < b.N; i++ {
		cache.Set(key(i), value(), 120)
	}
}

func BenchmarkProbeLFUSet(b *testing.B) {
	cache := initProbeLfu(b.N)
	for i := 0; i < b.N; i++ {
		cache.Set(key(i), value(), 120)
	}
}

func BenchmarkProbeTTLSet(b *testing.B) {
	cache := initProbeTTL(b.N)
	defer cache.Close()
	for i := 0; i < b.N; i++ {
		cache.Set(key(i), value(), 120)
	}
}

// ------------------------------------------------------------------------------------------------

func BenchmarkMapGet(b *testing.B) {
	b.StopTimer()
	m := make(map[string][]byte)
	for i := 0; i < b.N; i++ {
		m[key(i)] = value()
	}

	b.StartTimer()
	hitCount := 0
	for i := 0; i < b.N; i++ {
		if m[key(i)] != nil {
			hitCount++
		}
	}
}

func BenchmarkConcurrentMapGet(b *testing.B) {
	b.StopTimer()
	var m sync.Map
	for i := 0; i < b.N; i++ {
		m.Store(key(i), value())
	}

	b.StartTimer()
	hitCounter := 0
	for i := 0; i < b.N; i++ {
		_, ok := m.Load(key(i))
		if ok {
			hitCounter++
		}
	}
}

func BenchmarkFreeCacheGet(b *testing.B) {
	b.StopTimer()
	cache := freecache.NewCache(b.N * maxEntrySize)
	for i := 0; i < b.N; i++ {
		cache.Set([]byte(key(i)), value(), 0)
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		cache.Get([]byte(key(i)))
	}
}

func BenchmarkBigCacheGet(b *testing.B) {
	b.StopTimer()
	cache := initBigCache(b.N)
	for i := 0; i < b.N; i++ {
		cache.Set(key(i), value())
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(key(i))
	}
}

func BenchmarkProbeLRUGet(b *testing.B) {
	b.StopTimer()
	cache := initProbeLru(b.N)
	for i := 0; i < b.N; i++ {
		cache.Set(key(i), value(), 120)
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(key(i))
	}
}

func BenchmarkProbeLFUGet(b *testing.B) {
	b.StopTimer()
	cache := initProbeLfu(b.N)
	for i := 0; i < b.N; i++ {
		cache.Set(key(i), value(), 120)
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(key(i))
	}
}

func BenchmarkProbeTTLGet(b *testing.B) {
	b.StopTimer()
	cache := initProbeTTL(b.N)
	defer cache.Close()
	for i := 0; i < b.N; i++ {
		cache.Set(key(i), value(), 120)
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(key(i))
	}
}

// ------------------------------------------------------------------------------------------------

func BenchmarkBigCacheSetParallel(b *testing.B) {
	cache := initBigCache(b.N)
	rand.Seed(time.Now().Unix())

	b.RunParallel(func(pb *testing.PB) {
		id := rand.Intn(1000)
		counter := 0
		for pb.Next() {
			cache.Set(parallelKey(id, counter), value())
			counter = counter + 1
		}
	})
}

func BenchmarkFreeCacheSetParallel(b *testing.B) {
	cache := freecache.NewCache(b.N * maxEntrySize)
	rand.Seed(time.Now().Unix())

	b.RunParallel(func(pb *testing.PB) {
		id := rand.Intn(1000)
		counter := 0
		for pb.Next() {
			cache.Set([]byte(parallelKey(id, counter)), value(), 0)
			counter = counter + 1
		}
	})
}

func BenchmarkConcurrentMapSetParallel(b *testing.B) {
	var m sync.Map

	b.RunParallel(func(pb *testing.PB) {
		id := rand.Intn(1000)
		for pb.Next() {
			m.Store(key(id), value())
		}
	})
}

func BenchmarkProbeLRUSetParallel(b *testing.B) {
	cache := initProbeLru(b.N)
	rand.Seed(time.Now().Unix())

	b.RunParallel(func(pb *testing.PB) {
		id := rand.Intn(1000)
		counter := 0
		for pb.Next() {
			cache.Set(parallelKey(id, counter), value(), 600)
			counter = counter + 1
		}
	})
}

func BenchmarkProbeLFUSetParallel(b *testing.B) {
	cache := initProbeLfu(b.N)
	rand.Seed(time.Now().Unix())

	b.RunParallel(func(pb *testing.PB) {
		id := rand.Intn(1000)
		counter := 0
		for pb.Next() {
			cache.Set(parallelKey(id, counter), value(), 600)
			counter = counter + 1
		}
	})
}

func BenchmarkProbeTTLSetParallel(b *testing.B) {
	cache := initProbeTTL(b.N)
	defer cache.Close()
	rand.Seed(time.Now().Unix())

	b.RunParallel(func(pb *testing.PB) {
		id := rand.Intn(1000)
		counter := 0
		for pb.Next() {
			cache.Set(parallelKey(id, counter), value(), 600)
			counter = counter + 1
		}
	})
}

// ----------------------------------------------

func BenchmarkBigCacheGetParallel(b *testing.B) {
	b.StopTimer()
	cache := initBigCache(b.N)
	for i := 0; i < b.N; i++ {
		cache.Set(key(i), value())
	}

	b.StartTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			cache.Get(key(counter))
			counter = counter + 1
		}
	})
}

func BenchmarkFreeCacheGetParallel(b *testing.B) {
	b.StopTimer()
	cache := freecache.NewCache(b.N * maxEntrySize)
	for i := 0; i < b.N; i++ {
		cache.Set([]byte(key(i)), value(), 0)
	}

	b.StartTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			cache.Get([]byte(key(counter)))
			counter = counter + 1
		}
	})
}

func BenchmarkConcurrentMapGetParallel(b *testing.B) {
	b.StopTimer()
	var m sync.Map
	for i := 0; i < b.N; i++ {
		m.Store(key(i), value())
	}

	b.StartTimer()
	hitCount := 0

	b.RunParallel(func(pb *testing.PB) {
		id := rand.Intn(1000)
		for pb.Next() {
			_, ok := m.Load(key(id))
			if ok {
				hitCount++
			}
		}
	})
}

func BenchmarkProbeLRUGetParallel(b *testing.B) {
	b.StopTimer()
	cache := initProbeLru(b.N)
	for i := 0; i < b.N; i++ {
		cache.Set(key(i), value(), 600)
	}

	b.StartTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			cache.Get(key(counter))
			counter = counter + 1
		}
	})
}

func BenchmarkProbeLFUGetParallel(b *testing.B) {
	b.StopTimer()
	cache := initProbeLfu(b.N)
	for i := 0; i < b.N; i++ {
		cache.Set(key(i), value(), 600)
	}

	b.StartTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			cache.Get(key(counter))
			counter = counter + 1
		}
	})
}

func BenchmarkProbeTTLGetParallel(b *testing.B) {
	b.StopTimer()
	cache := initProbeTTL(b.N)
	defer cache.Close()
	for i := 0; i < b.N; i++ {
		cache.Set(key(i), value(), 600)
	}

	b.StartTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			cache.Get(key(counter))
			counter = counter + 1
		}
	})
}

func key(i int) string {
	return fmt.Sprintf("key-%010d", i)
}

func value() []byte {
	return make([]byte, 100)
}

func parallelKey(threadID int, counter int) string {
	return fmt.Sprintf("key-%04d-%06d", threadID, counter)
}

func initBigCache(entriesInWindow int) *bigcache.BigCache {
	cache, _ := bigcache.NewBigCache(bigcache.Config{
		Shards:             256,
		LifeWindow:         10 * time.Minute,
		MaxEntriesInWindow: entriesInWindow,
		MaxEntrySize:       maxEntrySize,
		Verbose:            true,
	})

	return cache
}

func initProbeLru(maxEntries int) *probecache.LRUStorage {
	mem := maxEntries * maxEntrySize
	crit := int(float64(mem) * 1.2)
	cache, _ := probecache.NewLRUStorage(numShards, mem, crit, 7)
	return cache
}

func initProbeLfu(maxEntries int) *probecache.LFUStorage {
	mem := maxEntries * maxEntrySize
	crit := int(float64(mem) * 1.2)
	cache, _ := probecache.NewLFUStorage(numShards, mem, crit, 7)
	return cache
}

func initProbeTTL(maxEntries int) *probecache.TTLStorage {
	cache, _ := probecache.NewTTLStorage(numShards, 0)
	return cache
}
