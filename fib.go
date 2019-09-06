package main

import (
	"fmt"
	"math"
	"sync"
)

const storeSize = 1000000 // 1,000,000

// Struct for caching fibonacci numbers
type FibTracker struct {
	lastIndex       uint32
	store           [storeSize]uint64
	updatingStore   bool
	storeStatusLock *sync.Mutex
	lock            *sync.Mutex
	cond            *sync.Cond
}

// Create a new FibTracker
func MakeFibTracker() *FibTracker {
	statusLock := &sync.Mutex{}
	condLock := &sync.Mutex{}
	fib := &FibTracker{1, [storeSize]uint64{}, false, statusLock, condLock, sync.NewCond(condLock)}
	fib.store[0], fib.store[1] = 0, 1
	return fib
}

func (fib *FibTracker) WithInitializedStore(nInit uint32) *FibTracker {
	nInit = uint32(math.Min(math.Max(2, float64(nInit)), storeSize))
	for i := uint32(2); i < nInit; i++ {
		fib.store[i] = fib.store[i-1] + fib.store[i-2]
	}
	fib.lastIndex = nInit - 1

	return fib
}

// Increase the size of store
func (fib *FibTracker) extendStore(includeIndex uint32) {
	if fib.lastIndex+1 == storeSize {
		return
	}

	// check if someone else is already updating the store, or if it's been updated to include our index
	fib.storeStatusLock.Lock()
	if fib.updatingStore || fib.lastIndex >= includeIndex {
		fib.storeStatusLock.Unlock()
		return
	}
	fib.updatingStore = true
	fib.storeStatusLock.Unlock()

	// update store
	newLen := uint32(math.Min(float64(includeIndex*2), float64(storeSize)))
	fmt.Println(newLen)
	for i := fib.lastIndex + 1; i < newLen; i += 1 {
		fib.store[i] = fib.store[i-1] + fib.store[i-2]
	}

	fib.storeStatusLock.Lock()
	fib.lastIndex = newLen - 1
	fib.updatingStore = false
	fib.storeStatusLock.Unlock()
}

// calculate a fib number at targetIndex starting at startIdx
func (fib *FibTracker) calcFromIndex(startIdx uint32, targetIdx uint32) uint64 {
	var ni uint64
	n1, n2 := fib.store[startIdx-1], fib.store[startIdx-2]
	for i := startIdx; i <= targetIdx; i += 1 {
		ni = n1 + n2
		n2 = n1
		n1 = ni
	}

	return ni
}

// get value at idx in fibonacci sequence
func (fib *FibTracker) Get(idx uint32) uint64 {
	lastIndex := fib.lastIndex
	if idx <= lastIndex {
		return fib.store[idx]
	}

	if lastIndex+1 < storeSize {
		go fib.extendStore(idx)
	}

	return fib.calcFromIndex(lastIndex+1, idx)
}
