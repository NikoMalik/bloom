package bloom

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"unsafe"
)

func key(i uint64) uint64 { return i*0x9e3779b97f4a7c15 + 0xdeadbeef }

func TestAddContainsNoFalseNegatives(t *testing.T) {
	const n = 10_000
	bf := NewFilter(1 << 16)

	for i := uint64(0); i < n; i++ {
		bf.Add(key(i))
	}
	for i := uint64(0); i < n; i++ {
		if !bf.Contains(key(i)) {
			t.Fatalf("false negative for key %d", i)
		}
	}
}

func TestAddContainsBytesNoFalseNegatives(t *testing.T) {
	const n = 5_000
	bf := NewFilter(1 << 16)

	keys := make([][]byte, n)
	for i := range keys {
		buf := make([]byte, 16)
		binary.LittleEndian.PutUint64(buf[:8], uint64(i))
		binary.LittleEndian.PutUint64(buf[8:], ^uint64(i))
		keys[i] = buf
		bf.AddBytes(buf)
	}
	for i, k := range keys {
		if !bf.ContainsBytes(k) {
			t.Fatalf("false negative for bytes key %d", i)
		}
	}
}

func TestAddContainsStringNoFalseNegatives(t *testing.T) {
	const n = 5_000
	bf := NewFilter(1 << 16)

	keys := make([]string, n)
	for i := range keys {
		keys[i] = fmt.Sprintf("user:%d:session", i)
		bf.AddString(keys[i])
	}
	for i, k := range keys {
		if !bf.ContainsString(k) {
			t.Fatalf("false negative for string key %d", i)
		}
	}
}

func TestFalsePositiveRateIsSane(t *testing.T) {
	// m/n = 32, k = 6
	const n = 20_000
	const m = 32 * n

	bf := NewFilter(uint32(m))
	for i := uint64(0); i < n; i++ {
		bf.Add(key(i))
	}

	rng := rand.New(rand.NewSource(1))
	fp := 0
	const trials = 200_000
	for i := 0; i < trials; i++ {
		k := rng.Uint64() | (1 << 63) // no ours
		if bf.Contains(k) {
			fp++
		}
	}
	rate := float64(fp) / float64(trials)
	t.Logf("false positive rate: %.6f (%d/%d)", rate, fp, trials)
	if rate > 1e-3 {
		t.Fatalf("FPR too high: %f", rate)
	}
}

func TestClear(t *testing.T) {
	bf := NewFilter(1 << 12)
	for i := uint64(0); i < 1000; i++ {
		bf.Add(key(i))
	}
	bf.Clear()
	for i := uint64(0); i < 1000; i++ {
		if bf.Contains(key(i)) {
			// false positive accept
		}
	}
	if r := bf.FillRatio(); r > 0.05 {
		t.Fatalf("fillRatio after clear too high: %f", r)
	}
}

func TestFillRatioMonotonic(t *testing.T) {
	bf := NewFilter(1 << 16)
	prev := bf.FillRatio()
	if prev != 0 {
		t.Fatalf("expected empty filter, got %f", prev)
	}
	for i := uint64(0); i < 5_000; i++ {
		bf.Add(key(i))
		if i%500 == 0 {
			cur := bf.FillRatio()
			if cur < prev {
				t.Fatalf("fillRatio decreased: %f -> %f", prev, cur)
			}
			prev = cur
		}
	}
	if prev <= 0 {
		t.Fatal("fillRatio didn't grow")
	}
}

func TestZeroAndDefaultBitCount(t *testing.T) {
	bf := NewFilter(0)
	if bf.numBuckets == 0 {
		t.Fatal("numBuckets is zero")
	}
	bf.Add(42)
	if !bf.Contains(42) {
		t.Fatal("default-sized filter broken")
	}
}

func TestConcurrentAddContains(t *testing.T) {
	bf := NewFilter(1 << 20)

	const goroutines = 8
	const perG = 5_000

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg.Done()
			base := uint64(g) << 32
			for j := uint64(0); j < perG; j++ {
				k := base | j
				bf.Add(k)
				if !bf.Contains(k) {
					t.Errorf("false negative for g=%d j=%d", g, j)
					return
				}
			}
		}(g)
	}
	wg.Wait()
}

func TestConcurrentAddBytes(t *testing.T) {
	bf := NewFilter(1 << 20)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for j := 0; j < 2000; j++ {
				buf := make([]byte, 16)
				binary.LittleEndian.PutUint64(buf, uint64(g))
				binary.LittleEndian.PutUint64(buf[8:], uint64(j))
				bf.AddBytes(buf)
			}
		}(g)
	}
	wg.Wait()
}

func TestMixedKeyspaces(t *testing.T) {
	bf := NewFilter(1 << 16)
	bf.Add(12345)
	bf.AddBytes([]byte("hello"))
	bf.AddString("world")

	if !bf.Contains(12345) {
		t.Fatal("uint64 key lost")
	}
	if !bf.ContainsBytes([]byte("hello")) {
		t.Fatal("bytes key lost")
	}
	if !bf.ContainsString("world") {
		t.Fatal("string key lost")
	}
}

func TestDataAlignment(t *testing.T) {
	f := NewFilter(1 << 24)
	addr := uintptr(unsafe.Pointer(&f.data[0]))
	if addr%uintptr(cacheLineBytes) != 0 {
		t.Fatalf("data not cache-line aligned: addr=%x %% %d = %d",
			addr, cacheLineBytes, addr%uintptr(cacheLineBytes))
	}
}
