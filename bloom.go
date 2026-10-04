package bloom

import (
	"math/bits"
	"sync/atomic"
	"unsafe"

	"github.com/NikoMalik/fxhash"
	"golang.org/x/sys/cpu"
)

const cacheLineBytes uint32 = uint32(unsafe.Sizeof(cpu.CacheLinePad{}))
const cacheLineBits = cacheLineBytes * 8 // 512
const numProbes = 6
const bitShift = 32 - 9
const wordsPerBucket = cacheLineBytes / 8 //

const golden_ratio uint32 = 0x9e3779b9
const c1 uint64 = 0xbf58476d1ce4e5b9
const c2 uint64 = 0x94d049bb133111eb

func mix(x uint64) uint64 {
	var v = x
	v = (v ^ (v >> 30)) * c1
	v = (v ^ (v >> 27)) * c2
	v = v ^ (v >> 31)
	return v
}

const DEFAULT_BITCOUNT = 1024

type BloomFilter struct {
	data       []atomic.Uint64
	numBuckets uint32
	bucketMask uint32
}

func NewFilter(bitCount uint32) *BloomFilter {
	if bitCount == 0 {
		bitCount = DEFAULT_BITCOUNT
	}

	if !isPowerOfTwo(uint(bitCount)) {
		bitCount = uint32(nextPowerOfTwo(bitCount))
	}

	// cacheLineBits is power of two, so division is exact
	numBuckets := max(1, bitCount/cacheLineBits)
	var totalWords = numBuckets * wordsPerBucket

	var data = alignSlice[atomic.Uint64](int(totalWords), int(cacheLineBits))

	return &BloomFilter{
		data:       data,
		numBuckets: numBuckets,
		bucketMask: numBuckets - 1,
	}
}

func (b *BloomFilter) Add(key uint64) {
	b.addHash(fxhash.Fxhash(key, 0))
}

func (b *BloomFilter) AddBytes(data []byte) {
	f := fxhash.NewFxHasher(0)
	f.Update(data)
	b.addHash(f.Finish())
}

func (b *BloomFilter) ContainsBytes(data []byte) bool {
	f := fxhash.NewFxHasher(0)
	f.Update(data)
	return b.containsHash(f.Finish())
}

func (b *BloomFilter) AddString(data string) {
	f := fxhash.NewFxHasher(0)
	f.Update([]byte(data))
	b.addHash(f.Finish())
}

func (b *BloomFilter) ContainsString(data string) bool {
	f := fxhash.NewFxHasher(0)
	f.Update([]byte(data))
	return b.containsHash(f.Finish())
}

func (b *BloomFilter) addHash(mixed uint64) {
	h1 := uint32(mixed)
	// bucket := fastrange_u32(h1, b.numBuckets)
	bucket := h1 & b.bucketMask
	off := bucket * wordsPerBucket

	h := uint32(mixed >> 32)

	var words [8]uint64
	var touched uint8

	for range numProbes {
		bitpos := h >> bitShift
		wordIdx := bitpos >> 6
		bitIdx := bitpos & 63
		words[wordIdx] |= 1 << bitIdx
		touched |= 1 << wordIdx

		h ^= h >> 15
		h *= golden_ratio
		h ^= h >> 13
	}

	for touched != 0 {
		wordIdx := uint8(bits.TrailingZeros8(touched))
		b.data[off+uint32(wordIdx)].Or(words[wordIdx])
		touched &= touched - 1
	}
}

func (b *BloomFilter) containsHash(mixed uint64) bool {
	h1 := uint32(mixed)
	// bucket := fastrange_u32(h1, b.numBuckets)
	bucket := h1 & b.bucketMask

	off := bucket * wordsPerBucket

	h := uint32(mixed >> 32)

	for range numProbes {
		bitpos := h >> bitShift
		wordIdx := bitpos >> 6
		bitIdx := bitpos & 63
		mask := uint64(1) << bitIdx

		if b.data[off+wordIdx].Load()&mask == 0 {
			return false
		}

		h *= golden_ratio
	}
	return true
}

func (b *BloomFilter) Contains(key uint64) bool {
	return b.containsHash(fxhash.Fxhash(key, 0))
}

func (b *BloomFilter) Clear() {
	for i := range b.data {
		b.data[i].Store(0)
	}
}

func (b *BloomFilter) FillRatio() float64 {
	var setBits uint64
	for i := range b.data {
		setBits += uint64(bits.OnesCount64(b.data[i].Load()))
	}
	totalBits := uint64(len(b.data)) * 64
	return float64(setBits) / float64(totalBits)
}
