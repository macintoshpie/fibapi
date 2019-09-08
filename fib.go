package main

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"sync"

	"github.com/hashicorp/golang-lru"
)

// Struct for caching fibonacci numbers
// The cache stores pairs of indices and their corresponding fibonacci values
type FibTracker struct {
	cachePad uint32   // # of non-cached entries between each cached entry (ie caching interval)
	cache    fibCache // lru cache
}

type fibCache interface {
	Get(uint32) (fibPair, error)
	Set(uint32, fibPair) error
}

var notFound error = errors.New("Not in cache")

type hashicorpCache struct {
	cache *lru.Cache
}

func MakeHashicorpCache(size int) (fibCache, error) {
	hc, err := lru.New(size)
	if err != nil {
		return &hashicorpCache{}, err
	}
	return &hashicorpCache{hc}, nil
}

func (c *hashicorpCache) Get(idx uint32) (fibPair, error) {
	pair, ok := c.cache.Get(idx)
	if ok == false {
		return fibPair{}, notFound
	}

	return pair.(fibPair), nil
}

func (c *hashicorpCache) Set(idx uint32, pair fibPair) error {
	c.cache.Add(idx, pair)
	return nil
}

type scEntry struct {
	idx  uint32
	pair fibPair
}

type sliceCache []scEntry

func MakeSliceCache(size int) (fibCache, error) {
	var sc sliceCache = make([]scEntry, size)
	sc[0] = scEntry{0, fibPair{big.NewInt(0), big.NewInt(1)}}
	for i := 1; i < size; i += 1 {
		sc[i].pair = fibPair{big.NewInt(0), big.NewInt(0)}
	}
	return &sc, nil
}

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

func (c *sliceCache) Set(idx uint32, pair fibPair) error {
	scMux.Lock()
	defer scMux.Unlock()
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
	return &FibTracker{uint32(cachePad), cache}
}

// initialize fibtracker cache with some precalculated values
func (fib *FibTracker) WithInitializedStore(nInit int) *FibTracker {
	nInit = int(math.Max(2, float64(nInit)))
	n1 := big.NewInt(0)
	n2 := big.NewInt(1)
	nCached := 0
	i := uint32(2)
	for nCached < nInit {
		n2.Add(n2, n1)
		n1, n2 = n2, n1
		if i%fib.cachePad == 0 {
			fib.cache.Set(i, fibPair{n1, n2})
			nCached += 1
		}
		i += 1
	}
	return fib
}

var basePair = fibPair{big.NewInt(0), big.NewInt(1)}

func (fib *FibTracker) calcFromZero(idx uint32) *big.Int {
	basePair.i.SetInt64(0)
	basePair.j.SetInt64(1)
	return fib.calcFromPair(0, basePair, idx)
}

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

func (fib *FibTracker) roundDownToPad(idx uint32) uint32 {
	rounded := idx - (idx % fib.cachePad)
	if rounded > idx {
		// uint overflow
		return 0
	}
	return rounded
}

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
			// fmt.Println("Hit: idx")
			return pair.i
		}
	} else if (idx+1)%fib.cachePad == 0 {
		pair, err := fib.cache.Get(idx + 1)
		if err == nil {
			// fmt.Println("Hit: idx+1")
			return pair.j
		}
	}

	// try to get nearest value that's cached
	// TODO: improve this by using BST and finding first node < idx? Would probably require some locks
	closeIndex := fib.roundDownToPad(idx)
	for i := 0; i < 10 && closeIndex <= idx; i += 1 {
		pair, err := fib.cache.Get(closeIndex)
		if err == nil {
			// calculate our value from this pair
			// fmt.Printf("Hit: %v (original %v)\n", closeIndex, idx)
			return fib.calcFromPair(closeIndex, pair, idx)
		}
		closeIndex -= fib.cachePad
	}

	// failed to use cache, calculate from zero
	// fmt.Printf("MISS: %v\n", idx)
	return fib.calcFromZero(idx)
}
