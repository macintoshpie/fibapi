package main

import (
	"math"
	"math/big"
	"sync"
)

const storeSize = 1000000 // 1,000,000

// Struct for caching fibonacci numbers
type FibTracker struct {
	lastIndex       uint32
	store           [storeSize]*big.Int
	updatingStore   bool
	storeStatusLock *sync.Mutex
	lock            *sync.Mutex
	cond            *sync.Cond
}

// Create a new FibTracker
func MakeFibTracker() *FibTracker {
	statusLock := &sync.Mutex{}
	condLock := &sync.Mutex{}
	// initialize store
	store := [storeSize]*big.Int{}
	for i := 0; i < storeSize; i += 1 {
		store[i] = big.NewInt(0)
	}
	fib := &FibTracker{1, store, false, statusLock, condLock, sync.NewCond(condLock)}
	fib.store[1].SetInt64(1)
	return fib
}

func (fib *FibTracker) WithInitializedStore(nInit uint32) *FibTracker {
	nInit = uint32(math.Min(math.Max(2, float64(nInit)), storeSize))
	for i := uint32(2); i < nInit; i++ {
		fib.store[i].Add(fib.store[i-1], fib.store[i-2])
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
	newLen := uint32(math.Min(float64(includeIndex+3000), float64(storeSize)))
	for i := fib.lastIndex + 1; i < newLen; i += 1 {
		fib.store[i].Add(fib.store[i-1], fib.store[i-2])
	}

	fib.storeStatusLock.Lock()
	fib.lastIndex = newLen - 1
	fib.updatingStore = false
	fib.storeStatusLock.Unlock()
}

// calculate a fib number at targetIndex starting at startIdx using store
func (fib *FibTracker) calcFromIndex(startIdx uint32, targetIdx uint32) *big.Int {
	n1 := big.NewInt(0).Set(fib.store[startIdx-1])
	n2 := big.NewInt(0).Set(fib.store[startIdx-2])
	nIter := targetIdx - startIdx + 1
	for i := uint32(0); i < nIter; i++ {
		n2.Add(n2, n1)
		n1, n2 = n2, n1
	}
	return n1
}

// get value at idx in fibonacci sequence
func (fib *FibTracker) Get(idx uint32) *big.Int {
	lastIndex := fib.lastIndex
	if idx <= lastIndex {
		return fib.store[idx]
	}

	if lastIndex+1 < storeSize && fib.updatingStore == false {
		go fib.extendStore(idx)
	}

	// FIXME: wasteful - need to use condition variable to wait for update to finish then try again
	return fib.calcFromIndex(lastIndex+1, idx)
}
