package main

import (
	"math/big"
	"strconv"
	"testing"

	lru "github.com/hashicorp/golang-lru"
)

var fibTests = []struct {
	index    uint32
	expected string
}{
	{0, "0"},
	{1, "1"},
	{2, "1"},
	{3, "2"},
	{4, "3"},
	{5, "5"},
	{6, "8"},
	{7, "13"},
	{8, "21"},
	{9, "34"},
	{50, "12586269025"},
	{75, "2111485077978050"},
	{100, "354224848179261915075"},
	{1000, "43466557686937456435688527675040625802564660517371780402481729089536555417949051890403879840079255169295922593080322634775209689623239873322471161642996440906533187938298969649928516003704476137795166849228875"},
}

func TestCalcFromZero(t *testing.T) {
	testCachePad := uint32(10)
	hc, err := MakeHashicorpCache(100)
	if err != nil {
		t.Fatal(err)
	}
	fib, err := MakeFibTracker(int(testCachePad), hc)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range fibTests {
		t.Run(strconv.Itoa(int(tt.index)), func(t *testing.T) {
			val := fib.calcFromZero(tt.index)
			if val.String() != tt.expected {
				t.Fatalf("Expected (%v) Got (%v)", tt.expected, val)
			}

			// if our index was in the right interval, it should be cached with correct values
			if tt.index != 0 && tt.index%testCachePad == 0 {
				pair, err := fib.cache.Get(tt.index)
				if err != nil {
					t.Fatalf("Expected (idx %v to be in cache) Got (not in cache)", tt.index)
				}
				if pair.i.String() != tt.expected {
					t.Fatalf("Expected (%v) Got (%v)", tt.expected, pair.i)
				}
			}
		})
	}
}

// func TestGet(t *testing.T) {
// 	fib := MakeFibTracker()
// 	for _, tt := range fibGetTests {
// 		t.Run(strconv.Itoa(int(tt.index)), func(t *testing.T) {
// 			val := fib.Get(tt.index)
// 			if val.String() != tt.expected {
// 				t.Errorf("Expected (%v) Got (%v)", val, tt.expected)
// 			}
// 		})
// 	}
// }

var result *big.Int

func benchmarkSetNextN(nIter uint32, b *testing.B) {
	var hc fibCache
	var fib *FibTracker
	var err error
	var val *big.Int
	for n := 0; n < b.N; n++ {
		hc, err = MakeHashicorpCache(100)
		if err != nil {
			b.Fatal(err)
		}
		fib, err = MakeFibTracker(10, hc)
		if err != nil {
			b.Fatal(err)
		}
		if err != nil {
			b.Error(err)
		}
		for i := uint32(0); i < nIter; i++ {
			val = fib.Get(i)
		}
	}

	result = val
}

// func BenchmarkSetNext10(b *testing.B) {
// 	benchmarkSetNextN(10, b)
// }

// func BenchmarkSetNext100(b *testing.B) {
// 	benchmarkSetNextN(100, b)
// }

// func BenchmarkSetNext1000(b *testing.B) {
// 	benchmarkSetNextN(1000, b)
// }

// func BenchmarkSync(b *testing.B) {
// 	var s sync.Mutex
// 	for i := 0; i < b.N; i++ {
// 		s.Lock()
// 		s.Unlock()
// 	}
// }

func benchmarkHashicorpCacheN(size, nIter int, b *testing.B) {
	var c *lru.Cache
	var err error
	for n := 0; n < b.N; n++ {
		c, err = lru.New(size)
		if err != nil {
			b.Fatal(err)
		}
		for i := 0; i < nIter; i++ {
			c.Add(i, i)
		}
	}
}

func BenchmarkHashicorpCache10(b *testing.B) {
	benchmarkHashicorpCacheN(10, 10, b)
}

func BenchmarkHashicorpCache100(b *testing.B) {
	benchmarkHashicorpCacheN(10, 100, b)
}

func BenchmarkHashicorpCache1000(b *testing.B) {
	benchmarkHashicorpCacheN(10, 1000, b)
}
