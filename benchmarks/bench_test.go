package benchmarks

import (
	"encoding/binary"
	"hash"
	"testing"

	nikobloom "github.com/NikoMalik/bloom"
	"github.com/NikoMalik/fxhash"
	xxhash "github.com/cespare/xxhash/v2"
	phrozenbloom "github.com/phrozen/bloom"
)

const (
	numBits   = 10_000_000
	numProbes = 6
	preload   = 1 << 20 // 1_048_576
	missCount = 1 << 17 // 131_072
	keyMask   = preload - 1
	missMask  = missCount - 1
)

var (
	hitKeys  [][]byte
	missKeys [][]byte
)

func splitmix64(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

func makeKey(seed, i uint64) []byte {
	buf := make([]byte, 16)
	binary.LittleEndian.PutUint64(buf[:8], splitmix64(seed+i))
	binary.LittleEndian.PutUint64(buf[8:], splitmix64(seed+i+0xdeadbeef))
	return buf
}

func init() {
	hitKeys = make([][]byte, preload)
	for i := range hitKeys {
		hitKeys[i] = makeKey(0x1234567890abcdef, uint64(i))
	}
	missKeys = make([][]byte, missCount)
	for i := range missKeys {
		missKeys[i] = makeKey(0xfeedfacecafebabe, uint64(i))
	}
}

// ---------------------------------------------------------------- fxhash adapter

type fxHasherAdapter struct {
	h   fxhash.FxHasher
	sum uint64
	ok  bool
}

func newFxHasherAdapter() *fxHasherAdapter { return &fxHasherAdapter{} }

func (a *fxHasherAdapter) Reset() {
	a.h = fxhash.NewFxHasher(0)
	a.ok = false
	a.sum = 0
}

func (a *fxHasherAdapter) Write(p []byte) (int, error) {
	a.h.Update(p)
	a.ok = false
	return len(p), nil
}

func (a *fxHasherAdapter) Sum64() uint64 {
	if !a.ok {
		a.sum = a.h.Finish()
		a.ok = true
	}
	return a.sum
}

func (a *fxHasherAdapter) Sum(b []byte) []byte {
	v := a.Sum64()
	return append(b,
		byte(v>>56), byte(v>>48), byte(v>>40), byte(v>>32),
		byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

func (a *fxHasherAdapter) Size() int      { return 8 }
func (a *fxHasherAdapter) BlockSize() int { return 8 }

// ---------------------------------------------------------------- constructors

func newNiko() *nikobloom.BloomFilter {
	return nikobloom.NewFilter(numBits)
}

func newPhrozen() *phrozenbloom.Filter {
	return phrozenbloom.NewFilter(1<<24, numProbes)
}

func newPhrozenXXH3() *phrozenbloom.Filter {
	return phrozenbloom.NewFilter(1<<24, numProbes,
		phrozenbloom.WithHashFunc(func() hash.Hash64 { return xxhash.New() }))
}

func newPhrozenFx() *phrozenbloom.Filter {
	return phrozenbloom.NewFilter(1<<24, numProbes,
		phrozenbloom.WithHashFunc(func() hash.Hash64 { return newFxHasherAdapter() }))
}

// ---------------------------------------------------------------- equivalence

func TestFxHashEquivalence(t *testing.T) {
	data := hitKeys[0]

	niko := fxhash.NewFxHasher(0)
	niko.Update(data)
	want := niko.Finish()

	ad := newFxHasherAdapter()
	if _, err := ad.Write(data); err != nil {
		t.Fatal(err)
	}
	got := ad.Sum64()

	if got != want {
		t.Fatalf("hash mismatch: %x vs %x", got, want)
	}
}

// ---------------------------------------------------------------- NewFilter

func BenchmarkNewFilter_Niko(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = newNiko()
	}
}

func BenchmarkNewFilter_Phrozen(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = newPhrozen()
	}
}

// ---------------------------------------------------------------- Add

func BenchmarkAddBytes_Niko(b *testing.B) {
	f := newNiko()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.AddBytes(hitKeys[i&keyMask])
	}
}

func BenchmarkAddBytes_Phrozen(b *testing.B) {
	f := newPhrozen()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Add(hitKeys[i&keyMask])
	}
}

func BenchmarkAddBytes_Phrozen_Fx(b *testing.B) {
	f := newPhrozenFx()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Add(hitKeys[i&keyMask])
	}
}

func BenchmarkAddBytes_Phrozen_XXH3(b *testing.B) {
	f := newPhrozenXXH3()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Add(hitKeys[i&keyMask])
	}
}

// ---------------------------------------------------------------- Contains hit

func BenchmarkContainsHit_Niko(b *testing.B) {
	f := newNiko()
	for _, k := range hitKeys {
		f.AddBytes(k)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.ContainsBytes(hitKeys[i&keyMask])
	}
}

func BenchmarkContainsHit_Phrozen(b *testing.B) {
	f := newPhrozen()
	for _, k := range hitKeys {
		f.Add(k)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Contains(hitKeys[i&keyMask])
	}
}

func BenchmarkContainsHit_Phrozen_Fx(b *testing.B) {
	f := newPhrozenFx()
	for _, k := range hitKeys {
		f.Add(k)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Contains(hitKeys[i&keyMask])
	}
}

func BenchmarkContainsHit_Phrozen_XXH3(b *testing.B) {
	f := newPhrozenXXH3()
	for _, k := range hitKeys {
		f.Add(k)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Contains(hitKeys[i&keyMask])
	}
}

// ---------------------------------------------------------------- Contains miss

func BenchmarkContainsMiss_Niko(b *testing.B) {
	f := newNiko()
	for _, k := range hitKeys {
		f.AddBytes(k)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.ContainsBytes(missKeys[i&missMask])
	}
}

func BenchmarkContainsMiss_Phrozen(b *testing.B) {
	f := newPhrozen()
	for _, k := range hitKeys {
		f.Add(k)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Contains(missKeys[i&missMask])
	}
}

func BenchmarkContainsMiss_Phrozen_Fx(b *testing.B) {
	f := newPhrozenFx()
	for _, k := range hitKeys {
		f.Add(k)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Contains(missKeys[i&missMask])
	}
}

func BenchmarkContainsMiss_Phrozen_XXH3(b *testing.B) {
	f := newPhrozenXXH3()
	for _, k := range hitKeys {
		f.Add(k)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Contains(missKeys[i&missMask])
	}
}

// ---------------------------------------------------------------- Parallel Add

func BenchmarkAddParallel_Niko(b *testing.B) {
	f := newNiko()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := uint64(0)
		for pb.Next() {
			f.AddBytes(hitKeys[i&keyMask])
			i++
		}
	})
}

func BenchmarkAddParallel_Phrozen(b *testing.B) {
	f := newPhrozen()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := uint64(0)
		for pb.Next() {
			f.Add(hitKeys[i&keyMask])
			i++
		}
	})
}

func BenchmarkAddParallel_Phrozen_Fx(b *testing.B) {
	f := newPhrozenFx()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := uint64(0)
		for pb.Next() {
			f.Add(hitKeys[i&keyMask])
			i++
		}
	})
}

func BenchmarkAddParallel_Phrozen_XXH3(b *testing.B) {
	f := newPhrozenXXH3()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := uint64(0)
		for pb.Next() {
			f.Add(hitKeys[i&keyMask])
			i++
		}
	})
}

// ---------------------------------------------------------------- Parallel Contains

func BenchmarkContainsParallel_Niko(b *testing.B) {
	f := newNiko()
	for _, k := range hitKeys {
		f.AddBytes(k)
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := uint64(0)
		for pb.Next() {
			f.ContainsBytes(hitKeys[i&keyMask])
			i++
		}
	})
}

func BenchmarkContainsParallel_Phrozen(b *testing.B) {
	f := newPhrozen()
	for _, k := range hitKeys {
		f.Add(k)
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := uint64(0)
		for pb.Next() {
			f.Contains(hitKeys[i&keyMask])
			i++
		}
	})
}

func BenchmarkContainsParallel_Phrozen_Fx(b *testing.B) {
	f := newPhrozenFx()
	for _, k := range hitKeys {
		f.Add(k)
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := uint64(0)
		for pb.Next() {
			f.Contains(hitKeys[i&keyMask])
			i++
		}
	})
}

func BenchmarkContainsParallel_Phrozen_XXH3(b *testing.B) {
	f := newPhrozenXXH3()
	for _, k := range hitKeys {
		f.Add(k)
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := uint64(0)
		for pb.Next() {
			f.Contains(hitKeys[i&keyMask])
			i++
		}
	})
}

// ---------------------------------------------------------------- FPR

func TestFalsePositiveRate(t *testing.T) {
	const trials = 200_000

	niko := newNiko()
	phroz := newPhrozen()
	phrozFx := newPhrozenFx()
	phrozXXH3 := newPhrozenXXH3()

	for _, k := range hitKeys {
		niko.AddBytes(k)
		phroz.Add(k)
		phrozFx.Add(k)
		phrozXXH3.Add(k)
	}

	count := func(f func([]byte) bool) int {
		fp := 0
		for i := 0; i < trials; i++ {
			if f(missKeys[i&missMask]) {
				fp++
			}
		}
		return fp
	}

	t.Logf("Niko          FPR: %.6f", float64(count(niko.ContainsBytes))/trials)
	t.Logf("Phrozen FNV   FPR: %.6f", float64(count(phroz.Contains))/trials)
	t.Logf("Phrozen Fx    FPR: %.6f", float64(count(phrozFx.Contains))/trials)
	t.Logf("Phrozen XXH3  FPR: %.6f", float64(count(phrozXXH3.Contains))/trials)
}
