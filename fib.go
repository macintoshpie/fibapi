package main

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"sync"
	"sync/atomic"
)

// Struct for caching fibonacci numbers
// The cache stores pairs of indices and their corresponding fibonacci values
type FibTracker struct {
	cachePad   uint32     // # of non-cached entries between each cached entry (ie caching interval)
	CacheStats CacheStats // tracks cache hit/miss counts
	cache      fibCache   // lru cache
}

// interface for a cache usable by the FibTracker
type fibCache interface {
	Get(uint32) (fibPair, error)
	Set(uint32, fibPair) error
}

type CacheStats struct {
	NDirectHit uint64 `json:"direct"` // tracks number of direct cache hits (no calculation needed)
	NCloseHit  uint64 `json:"close"`  // tracks number of close cache hits (some calculation needed)
	NMiss      uint64 `json:"miss"`   // tracks number of cache misses (full calculation needed)
}

var notFound error = errors.New("Not in cache")

// Simple cache (a slice)
type sliceCache []scEntry

type scEntry struct {
	idx  uint32
	pair fibPair
}

// create a new slice cache
func MakeSliceCache(size int) (fibCache, error) {
	var sc sliceCache = make([]scEntry, size)
	sc[0] = scEntry{0, fibPair{big.NewInt(0), big.NewInt(1)}}
	for i := 1; i < size; i += 1 {
		sc[i].pair = fibPair{big.NewInt(0), big.NewInt(0)}
	}
	return &sc, nil
}

// TODO: refactor sliceCache to include mux
var scMux = &sync.Mutex{}

func (c *sliceCache) Get(idx uint32) (fibPair, error) {
	scMux.Lock()
	defer scMux.Unlock()
	entry := (*c)[idx%uint32(len(*c))]
	if entry.idx == idx {
		return entry.pair, nil
	}
	return fibPair{}, notFound
}

// Set key idx to pair
func (c *sliceCache) Set(idx uint32, pair fibPair) error {
	scMux.Lock()
	defer scMux.Unlock()
	// TODO: add a comparison of idx - if already equal there's no need to update
	(*c)[idx%uint32(len(*c))].pair.i.Set(pair.i)
	(*c)[idx%uint32(len(*c))].pair.j.Set(pair.j)
	(*c)[idx%uint32(len(*c))].idx = idx

	return nil
}

// stores fibonacci values for the ith and i+1th positions
type fibPair struct {
	i *big.Int // ith position
	j *big.Int // i+1th position
}

// Create a new FibTracker
func MakeFibTracker(cachePad int, cache fibCache) *FibTracker {
	return &FibTracker{
		cachePad:   uint32(cachePad),
		CacheStats: CacheStats{0, 0, 0},
		cache:      cache,
	}
}

func (fib *FibTracker) countHit() {
	atomic.AddUint64(&fib.CacheStats.NDirectHit, 1)
}

func (fib *FibTracker) countClose() {
	atomic.AddUint64(&fib.CacheStats.NCloseHit, 1)
}

func (fib *FibTracker) countMiss() {
	atomic.AddUint64(&fib.CacheStats.NMiss, 1)
}

// initialize fibtracker cache with some precalculated values
func (fib *FibTracker) WithInitializedStore(nInit uint32) *FibTracker {
	nInit = uint32(math.Max(2, float64(nInit)))
	fib.Get(nInit)
	return fib
}

var basePair = fibPair{big.NewInt(0), big.NewInt(1)}

// calculate fib number starting from 0 and 1
func (fib *FibTracker) calcFromZero(idx uint32) *big.Int {
	basePair.i.SetInt64(0)
	basePair.j.SetInt64(1)
	return fib.calcFromPair(0, basePair, idx)
}

// calculate fib number starting from provided pair
func (fib *FibTracker) calcFromPair(pairIdx uint32, pair fibPair, idx uint32) *big.Int {
	// check if idx is actually in our pair
	if pairIdx == idx {
		return pair.i
	} else if pairIdx-1 == idx {
		return pair.j
	}

	// calculate idx, caching as we go
	n1 := big.NewInt(0).Set(pair.i)
	n2 := big.NewInt(0).Set(pair.j)
	for i := pairIdx + 1; i < idx+1; i++ {
		// TODO:
		// depending on how large idx is (ie how expensive it is to calculate from zero),
		// cache some number of values on our way UP to idx
		n2.Add(n2, n1)
		n1, n2 = n2, n1
		if i%fib.cachePad == 0 {
			// TODO: call set i / fib.cachePad to make sure we use all indices of the cache
			fib.cache.Set(i, fibPair{n1, n2})
		}
	}
	return n1
}

// round idx down to nearest cachePad interval
func (fib *FibTracker) roundDownToPad(idx uint32) uint32 {
	rounded := idx - (idx % fib.cachePad)
	if rounded > idx {
		return 0
	}
	return rounded
}

// print contents of cache, useful for debugging
func (fib *FibTracker) printCache() {
	for i, val := range *fib.cache.(*sliceCache) {
		fmt.Printf("%v: %v (%v, %v)\n", i, val.idx, val.pair.i, val.pair.j)
	}
}

// get value at idx in fibonacci sequence
func (fib *FibTracker) Get(idx uint32) *big.Int {
	// try to get cached value (for i or i+1)
	if idx%fib.cachePad == 0 {
		pair, err := fib.cache.Get(idx)
		if err == nil {
			fib.countHit()
			return pair.i
		}
	} else if (idx+1)%fib.cachePad == 0 {
		pair, err := fib.cache.Get(idx + 1)
		if err == nil {
			fib.countHit()
			return pair.j
		}
	}

	// try to get nearest pair that's cached and calculate from their values
	// TODO: consider using BST to store cached values and finding first node < idx
	closeIndex := fib.roundDownToPad(idx)
	for i := 0; i < 10 && closeIndex <= idx; i += 1 {
		pair, err := fib.cache.Get(closeIndex)
		if err == nil {
			fib.countClose()
			return fib.calcFromPair(closeIndex, pair, idx)
		}
		closeIndex -= fib.cachePad
	}

	// failed to use cache, calculate from zero
	fib.countMiss()
	return fib.calcFromZero(idx)
}
