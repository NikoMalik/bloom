package bloom

import (
	"encoding/binary"
	"testing"
)

func BenchmarkAdd(b *testing.B) {
	bf := NewFilter(1 << 20)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bf.Add(key(uint64(i)))
	}
}

func BenchmarkContainsHit(b *testing.B) {
	const preload = 100_000
	bf := NewFilter(1 << 22)
	for i := uint64(0); i < preload; i++ {
		bf.Add(key(i))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bf.Contains(key(uint64(i % preload)))
	}
}

func BenchmarkContainsMiss(b *testing.B) {
	bf := NewFilter(1 << 22)
	for i := uint64(0); i < 100_000; i++ {
		bf.Add(key(i))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bf.Contains(key(uint64(i) | 1<<63))
	}
}

func BenchmarkAddParallel(b *testing.B) {
	bf := NewFilter(1 << 24)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := uint64(0)
		for pb.Next() {
			bf.Add(key(i))
			i++
		}
	})
}

func BenchmarkContainsParallel(b *testing.B) {
	const preload = 200_000
	bf := NewFilter(1 << 24)
	for i := uint64(0); i < preload; i++ {
		bf.Add(key(i))
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := uint64(0)
		for pb.Next() {
			bf.Contains(key(i % preload))
			i++
		}
	})
}

func BenchmarkAddBytes(b *testing.B) {
	bf := NewFilter(1 << 20)
	buf := make([]byte, 16)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		binary.LittleEndian.PutUint64(buf, uint64(i))
		bf.AddBytes(buf)
	}
}

func BenchmarkAddString(b *testing.B) {
	bf := NewFilter(1 << 20)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bf.AddString("user:12345:session:abcdef")
	}
}

func BenchmarkContainsBytes(b *testing.B) {
	bf := NewFilter(1 << 22)
	const preload = 100_000
	for i := uint64(0); i < preload; i++ {
		buf := make([]byte, 16)
		binary.LittleEndian.PutUint64(buf, uint64(i))
		bf.AddBytes(buf)
	}
	buf := make([]byte, 16)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		binary.LittleEndian.PutUint64(buf, uint64(i%preload))
		bf.ContainsBytes(buf)
	}
}

func BenchmarkNewFilterSmall(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = NewFilter(1 << 12)
	}
}
