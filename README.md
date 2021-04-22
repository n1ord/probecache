# Probe-cache

Набор кешеров на Map+RWMutex шардах, с использованием псеводслучайного LRU/LFU вытеснения с ограничением. 
Реализовано три хранилища: LFUStorage, LRUStorage и TTLStorage

# TTLStorage

Это простой неограниченный кеш с временем жизни у записей. Устаревающие записи удаляются фоновым сканером раз в N секунд (настраивается).
Хранилище шардировано, чистые мапы, быстрые параллельные Get'ы, Set'ы. Добавлено для общей совместимости.

# LFUStorage/LRUStorage

Ограниченный кеш с временем жизни у записей и псевдослучайным LRU/LFU вытеснением. Без фоновых процессов,
дополнительных структур данных и с константной сложностью на все операции.

Для кеша задается четыре параметра: 
- Число шардов
- Оптимальный расход памяти (порог №1)
- Максимальный расход памяти (порог №2)
- Число N задающее максимальное число итераций вытеснения

Далее:
1) Кеш делится на шарды, каждый шард - мапа с мьютексом. Входящие ключи хешируются и по хешу выбирается шард. 
2) Для каждой записи хранится инфа о ее "ценности" (число хитов записи или время последнего использования)
3) Каждый шард хранит инфу о суммарной и средней (по больнице) ценности всех своих элементов. Корректируется при Get/Set/Del элементов шарда 
4) Во время каждой SET операции, перед вставкой, в случае если объем кеша превышает порог №1, делается следующее:
    - последовательно выбирается N случайных ключей, для каждого ключа:
        - если ценность ключа меньше средней по шарду, ключ удаляется.
        - если объем кеша более не превышает порога, итерации прерываются
    - если дошли до N-й итерации, а объем кеша превышает порог №2 (максимальный расход памяти) - удаляется 2 случайных ключа

В чем суть - на больших данных с более менее нормальным распределением "ценности" (т.е. 80% запросов приходится на 20% кеша) - вероятность попасть в ключ,
который можно удалить - будет колебаться в районе 50% и выше. Чтобы память не росла, при вставке новых данных 
кеш должен удалять больше чем добавляет. Вероятность овер 50% означает, что в среднем для этого достаточно ткнуть в 2 случайных ключа 
(51% * 2 = 102%, т.е. кладем 1 ключ, удаляем 1.02).

В иных случаях - например, кеш небольшой, запросов мало, и данные запрашиваются равномерно без явных пиков, - вытесняется тупо 2 случайных ключа

Псевдослучайность выбора ключей основана на занятной особенности реализации range итерирования по мапе в GO (оно _вполне_ случайно для этой задачи)

**Профиты:**
+ все стабильно по памяти
+ константный оверхед Get/Set/Del операций
+ годный хитрейт на околонормально распределенной нагрузке на кеш
+ многопоточен (шарды, все дела), быстрые GET'ы
+ не заводит фоновых горутин-чистилок и прочего

**Минусы:**
- Запись в равномерно-нагруженный кеш (кеш запрашивается равномерно, без выраженных пиков) будет вытеснять случайные ключи, понижая хитрейт
- Относительно медленный SET (но с константной сложностью)

# Examples

**TTL**
```Go
numShards := 10
cleanPeriod := 1*time.Second
storage, err := pcache.NewTTLStorage(numShards, cleanPeriod)
if err != nil {
    panic(err)
}

// cache something for 120 sec
_ := storage.Set("key", []byte("value"), 120)

// get it from cache
if value, err := storage.Get("key"); err != nil {
    // HIT
} else {
    // MISS
}
```

**LRU/LFU**
```Go
numShards := 10
maxMemSize := 25 * 1024 * 1024    //25mb
critMemSize := 305 * 1024 * 1024  //30mb
maxDepth := 6                     //enough for most situations


storage, err := pcache.NewLRUStorage(numShards, maxMemSize, critMemSize, maxDepth)
// or 
//storage, err := pcache.NewLFUStorage(numShards, maxMemSize, critMemSize, maxDepth)
if err != nil {
    panic(err)
}

// cache something for 120 secначинаем вставку новых записей в кеш, где ключи уже накопили "ценность"

// get it from cache
if value, err := storage.Get("key"); err != nil {
    // HIT
} else {
    // MISS
}
```

# Бенчи

**Нагрузка и хитрейт**
- Кеш ограничен до ~5mb
- Чтение/запись производится из нормально распределенного множества ключей (пресловутые 80% запросов на 20% ключей)
- Одновременно ведется запись в другое множество ключей (без чтения)
- Прогрев (два последних теста) - первые 5 секунд из кеша производится только чтение

```
LFUStorage testing
Size: 50000343 bytes
Writes: 7635782 
Reads: 5852971 
Hitrate: 69%
Cache size: 48828kb / 48828kb / 58593kb

LRUStorage testing
Size: 50002468 bytes
Writes: 7097687 
Reads: 5710800 
Hitrate: 75%
Cache size: 48830kb / 48828kb / 58593kb

LFUStorage testing with warming
Size: 50000684 bytes
Writes: 6624513 
Reads: 5728529 
Hitrate: 84%
Cache size: 48828kb / 48828kb / 58593kb

LRUStorage testing with warming
Size: 50003748 bytes
Writes: 5989472 
Reads: 5443711 
Hitrate: 89%
Cache size: 48831kb / 48828kb / 58593kb
```

**Производительнсть:**
```
$ go test -bench=. -benchmem -benchtime=2s cmd/bench/cache_bench_test.go -timeout 30m
goos: linux
goarch: amd64
BenchmarkMapSet-4                     	 3661824	       668 ns/op	     238 B/op	       2 allocs/op
BenchmarkConcurrentMapSet-4           	 1257823	      1936 ns/op	     322 B/op	       8 allocs/op
BenchmarkFreeCacheSet-4               	 2685824	      1038 ns/op	     334 B/op	       2 allocs/op
BenchmarkBigCacheSet-4                	 2558430	       948 ns/op	     330 B/op	       2 allocs/op
BenchmarkProbeTTLSet-4                	 2257016	      1048 ns/op	     250 B/op	       3 allocs/op
BenchmarkProbeLRUSet-4                	 2162376	      1075 ns/op	     270 B/op	       3 allocs/op
BenchmarkProbeLFUSet-4                	 2221588	      1080 ns/op	     268 B/op	       3 allocs/op
BenchmarkMapGet-4                     	 5478710	       455 ns/op	      23 B/op	       1 allocs/op
BenchmarkConcurrentMapGet-4           	 3933380	       613 ns/op	      23 B/op	       1 allocs/op
BenchmarkFreeCacheGet-4               	 2679570	      1004 ns/op	     135 B/op	       2 allocs/op
BenchmarkBigCacheGet-4                	 3231510	       768 ns/op	     151 B/op	       3 allocs/op
BenchmarkProbeLRUGet-4                	 2713260	       950 ns/op	      23 B/op	       1 allocs/op
BenchmarkProbeLFUGet-4                	 2948250	       830 ns/op	      23 B/op	       1 allocs/op
BenchmarkProbeTTLGet-4                	 3231793	       772 ns/op	      23 B/op	       1 allocs/op
BenchmarkBigCacheSetParallel-4        	 5116780	       496 ns/op	     337 B/op	       3 allocs/op
BenchmarkFreeCacheSetParallel-4       	 5036130	       533 ns/op	     341 B/op	       3 allocs/op
BenchmarkConcurrentMapSetParallel-4   	 5466741	       458 ns/op	     196 B/op	       5 allocs/op
BenchmarkProbeLRUSetParallel-4        	 3978388	       620 ns/op	     285 B/op	       3 allocs/op
BenchmarkProbeLFUSetParallel-4        	 3811774	       650 ns/op	     293 B/op	       4 allocs/op
BenchmarkProbeTTLSetParallel-4        	 3647398	       762 ns/op	     279 B/op	       3 allocs/op
BenchmarkBigCacheGetParallel-4        	10068198	       349 ns/op	     151 B/op	       3 allocs/op
BenchmarkFreeCacheGetParallel-4       	 7099514	       431 ns/op	     135 B/op	       2 allocs/op
BenchmarkConcurrentMapGetParallel-4   	 6080289	       605 ns/op	      20 B/op	       1 allocs/op
BenchmarkProbeLRUGetParallel-4        	 6466082	       456 ns/op	      23 B/op	       1 allocs/op
BenchmarkProbeLFUGetParallel-4        	 6930259	       351 ns/op	      23 B/op	       1 allocs/op
BenchmarkProbeTTLGetParallel-4        	11634008	       256 ns/op	      23 B/op	       1 allocs/op
PASS
ok  	command-line-arguments	204.480s
```
