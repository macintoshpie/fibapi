package main

import (
	"math/big"
	"strconv"
	"testing"
)

var fibGetTests = []struct {
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

func TestGet(t *testing.T) {
	fib := MakeFibTracker()
	for _, tt := range fibGetTests {
		t.Run(strconv.Itoa(int(tt.index)), func(t *testing.T) {
			val := fib.Get(tt.index)
			if val.String() != tt.expected {
				t.Errorf("Expected (%v) Got (%v)", val, tt.expected)
			}
		})
	}
}

var result *big.Int

func benchmarkSetNextN(nIter uint32, b *testing.B) {
	var fib *FibTracker
	var val *big.Int
	for n := 0; n < b.N; n++ {
		fib = MakeFibTracker().WithInitializedStore(100000)
		for i := uint32(0); i < nIter; i++ {
			val = fib.Get(i)
		}
	}

	result = val
}

func BenchmarkSetNext1000(b *testing.B) {
	benchmarkSetNextN(1000, b)
}

func BenchmarkSetNext10000(b *testing.B) {
	benchmarkSetNextN(10000, b)
}

func BenchmarkSetNext100000(b *testing.B) {
	benchmarkSetNextN(100000, b)
}
