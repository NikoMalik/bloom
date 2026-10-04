package bloom

import (
	"math/bits"
	"unsafe"
)

func nextPowerOfTwo(i uint32) uint32 {
	i--
	i |= i >> 1
	i |= i >> 2
	i |= i >> 4
	i |= i >> 8
	i |= i >> 16
	i |= i >> 32
	i++
	return i
}

func alignSlice[T any](n, align int) []T {
	if n <= 0 {
		return nil
	}

	var zero T
	size := int(unsafe.Sizeof(zero))

	if size == 0 || align <= 0 || align%size != 0 {
		panic("invalid alignment")
	}

	pad := align/size - 1
	s := make([]T, n+pad)

	p := uintptr(unsafe.Pointer(&s[0]))
	aligned := (p + uintptr(align-1)) &^ uintptr(align-1)

	i := int((aligned - p) / uintptr(size))

	return s[i : i+n : i+n]
}

// fast analog for word % p
// https://lemire.me/blog/2016/06/27/a-fast-alternative-to-the-modulo-reduction/
func fastrange(word, p uint64) uint64 {
	hi, _ := bits.Mul64(word, p)
	return hi
}

func isPowerOfTwo(x uint) bool {
	return x&(x-1) == 0
}

func fastrange_u32(word, p uint32) uint32 {
	hi, _ := bits.Mul32(word, p)
	return hi
}

//go:linkname memmove runtime.memmove
func memmove(dst, src unsafe.Pointer, n uintptr)

//go:nocheckptr
func copyUnsafe[T any](dst []T, src []T) int {
	if len(dst) == 0 || len(src) == 0 {
		return 0
	}
	if len(src) > len(dst) {
		src = src[:len(dst)]
	}
	memmove(
		unsafe.Pointer(unsafe.SliceData(dst)),
		unsafe.Pointer(unsafe.SliceData(src)),
		uintptr(len(src))*unsafe.Sizeof(src[0]),
	)
	return len(src)
}
