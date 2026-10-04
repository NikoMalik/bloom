# bloom

A high-performance, cache-line–local Bloom filter for Go, designed for concurrent workloads with zero allocations on the hot path.

## Features

- **Cache-line-local probes.** All `k` probes for a key land inside a single 64/128-byte cache line. One memory fetch per operation.
- **Lock-free.** Backed by `[]atomic.Uint64`. Concurrent `Add` and `Contains` are safe without a mutex.
- **Zero allocations.** `Add`, `AddBytes`, `AddString`, `Contains`, `ContainsBytes`, `ContainsString` do not allocate.
- **Batched atomic OR.** Probes are accumulated in registers and flushed once per unique word, cutting locked RMWs by ~30%.
- **Inline hashing.** `fxhash` is monomorphized into the hot path — no `hash.Hash64` interface dispatch

## Status

The core design is stable. A few API details are still being polished:

- `AddString`/`ContainsString` currently allocate via `[]byte(string)`. Use `AddBytes`/`ContainsBytes` for the fastest path, or apply the `unsafe.Slice(unsafe.StringData(s), len(s))` fix.
- `NewFilter` rounds `bitCount` up to a power of two. This gives a cheap `bucketMask` on the hot path but can use up to 2× more memory than requested. See [Memory](#memory) below.
- `numProbes` is currently fixed at 6

## Usage

```go
package main

import (
    "fmt"

    "github.com/NikoMalik/bloom"
)

func main() {
    // 10M bits, rounded up internally to a power of two.
    f := bloom.NewFilter(10_000_000)

    f.Add(0xdeadbeef)
    f.AddBytes([]byte("hello"))
    f.AddString("world")

    fmt.Println(f.Contains(0xdeadbeef))            // true
    fmt.Println(f.ContainsBytes([]byte("hello")))  // true
    fmt.Println(f.ContainsString("world"))         // true
    fmt.Println(f.ContainsString("nope"))          // likely false

    fmt.Printf("fill ratio: %.4f\n", f.FillRatio())

    f.Clear()
}
```


## Design

### Cache-line-local bucket

The filter is split into buckets, each cacheLineBytes wide (64 on x86, 128 on Apple Silicon). A key is hashed to:

1. A bucket index - h1 & bucketMask.
2. A starting bit position inside the bucket - h2 >> (32-9), which yields 9 bits covering a 512-bit (64-byte) window.

Subsequent probes advance the 32-bit state with a xor-shift-multiply-xor-shift step:

    h ^= h >> 15
    h *= golden_ratio
    h ^= h >> 13

Then reuse the top 9 bits. Because the bucket is exactly one cache line, all probes hit the same line - one L1 fetch per operation, no pointer chasing.


### No interface in the hot path

fxhash is called directly. hash.Hash64 would add ~30-40 ns per operation due to interface dispatch

## Benchmarks

GOOS=linux 12 logical cores. Compared against github.com/phrozen/bloom

Both filters use m = 16_777_216 bits (1<<24) and k = 6 probes.

### Throughput (single-threaded)

    Benchmark                bloom      phrozen(FNV)  phrozen(fxhash)  phrozen(XXH3)
    AddBytes                 44.4 ns    88.4 ns       81.5 ns          92.9 ns
    ContainsHit              18.4 ns    65.3 ns       55.9 ns          65.1 ns
    ContainsMiss             28.3 ns    72.6 ns       62.2 ns          71.7 ns

### Throughput (parallel, GOMAXPROCS=12)

    Benchmark                bloom      phrozen(FNV)  phrozen(fxhash)  phrozen(XXH3)
    AddParallel              10.8 ns    19.1 ns       18.0 ns          18.7 ns
    ContainsParallel         2.88 ns    9.42 ns       8.87 ns          9.35 ns

### Construction

    Benchmark                bloom              phrozen
    NewFilter                132 us / 2 allocs  142 us / 3 allocs

### False positive rate

Same filter configuration (m = 1<<24, k = 6, 1M preloaded keys, 200k independent miss trials).

    Implementation     FPR
    bloom              0.0950%
    phrozen (FNV)      0.0935%
    phrozen (fxhash)   0.0810%
    phrozen (XXH3)     0.0845%

All implementations sit within 1.1-1.3x of the theoretical bound (1 - e^(-kn/m))^k = 0.076%. The spread is within ~2 sigma of sampling noise at 200k trials.




## Alternatives

- github.com/phrozen/bloom - same cache-line-local design, more flexible hashing, slower hot path.
- github.com/bits-and-blooms/bloom - classic non-concurrent implementation.
- github.com/tylertreat/BoomFilters - a wider family of probabilistic data structures.

## License

MIT. See LICENSE.
