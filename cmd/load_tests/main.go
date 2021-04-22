package main

import (
	"fmt"
	"math/rand"
	"time"

	pcache "github.com/n1ord/probecache"
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func BenchNormalLoad(storage pcache.IStorage, N int, maxValueSize int, warmDuration time.Duration, loadDuration time.Duration) {
	var started time.Time
	if warmDuration > 0 {
		for i := 0; i < N; i++ {
			key := fmt.Sprintf("%d", i)
			value := RandStringRunes(rand.Intn(maxValueSize-1) + 1)
			storage.Set(key, []byte(value), 120)
		}

		started = time.Now()
		for {
			r := int64(rand.NormFloat64()*float64(N)/6. + float64(N)/2)
			key := fmt.Sprintf("%d", r)
			storage.Get(key)

			if time.Since(started) > warmDuration {
				break
			}
		}
	}
	// GO Probabilistic-Negation Invalidation Cache
	// os.Exit(1)
	hits := 0.
	misses := 0.
	writes := 0
	reads := 0
	writeProb := 1.
	started = time.Now()
	for {
		// r := int64(rand.Float64() * float64(N))
		r := int64(rand.NormFloat64()*float64(N)/6. + float64(N)/2)
		key := fmt.Sprintf("%d", r)
		_, err := storage.Get(key)
		reads++
		if err != nil {
			writes++
			storage.Set(key, []byte("somevalue"), 120)
			misses++
		} else {
			hits++
		}

		if rand.Float32() < float32(writeProb) {
			key = fmt.Sprintf("randkey%f", rand.Float32())
			value := RandStringRunes(rand.Intn(maxValueSize-1) + 1)
			writes++
			storage.Set(key, []byte(value), 120)
		}

		if time.Since(started) > loadDuration {
			break
		}
		// if int32(elapsed)%10 == 0 {
		// 	fmt.Printf("Size: %d bytes\n", storage.GetSize())
		// }
	}
	fmt.Printf("Size: %d bytes\n", storage.GetSize())
	fmt.Printf("Writes: %d \n", writes)
	fmt.Printf("Reads: %d \n", reads)
	fmt.Printf("Hitrate: %d%%\n", int32(100.*hits/(hits+misses)))
	storage.PrintInfo()
}

func main() {
	N := 1000000
	maxValueSize := 50
	maxMemSize := maxValueSize * N
	cleanDepth := 5
	critMemSize := int(float64(maxMemSize) * 1.2)
	{
		fmt.Println("LFUStorage testing")
		storage, err := pcache.NewLFUStorage(10, maxMemSize, critMemSize, cleanDepth)
		if err != nil {
			panic(err)
		}
		BenchNormalLoad(storage, N, maxValueSize, 0*time.Second, 30*time.Second)
	}
	fmt.Println("")
	{
		fmt.Println("LRUStorage testing")
		storage, err := pcache.NewLRUStorage(10, maxMemSize, critMemSize, cleanDepth)
		if err != nil {
			panic(err)
		}
		BenchNormalLoad(storage, N, maxValueSize, 0*time.Second, 30*time.Second)
	}
	fmt.Println("")
	{
		fmt.Println("LFUStorage testing with warming")
		storage, err := pcache.NewLFUStorage(10, maxMemSize, critMemSize, cleanDepth)
		if err != nil {
			panic(err)
		}
		BenchNormalLoad(storage, N, maxValueSize, 5*time.Second, 30*time.Second)
	}
	fmt.Println("")
	{
		fmt.Println("LRUStorage testing with warming")
		storage, err := pcache.NewLRUStorage(10, maxMemSize, critMemSize, cleanDepth)
		if err != nil {
			panic(err)
		}
		BenchNormalLoad(storage, N, maxValueSize, 5*time.Second, 30*time.Second)
	}

	// fmt.Println("Writing")
	// writeDuration := 120.
	// started = time.Now()
	// for {
	// 	key := fmt.Sprintf("randkey2 %f", rand.Float32()*1000.)
	// 	storage.Set(key, value, 120)

	// 	elapsed := time.Since(started).Seconds()
	// 	if elapsed > writeDuration {
	// 		break
	// 	}
	// }
	// fmt.Printf("Size: %d bytes\n", storage.GetSize())
	// storage.PrintInfo()
}
