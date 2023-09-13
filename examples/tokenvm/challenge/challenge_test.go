// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package challenge

import (
	"fmt"
	"testing"
)

// Bench: go test -bench=. -benchtime=20x -benchmem
//
// goos: darwin
// goarch: arm64
// pkg: github.com/ava-labs/hypersdk/examples/tokenvm/challenge
// BenchmarkSearch/difficulty=22_cores=1-12         	      10	1856272250 ns/op	1099921195 B/op	17186238 allocs/op
// BenchmarkSearch/difficulty=22_cores=2-12         	      10	 530316125 ns/op	540141624 B/op	 8439696 allocs/op
// BenchmarkSearch/difficulty=22_cores=4-12         	      10	 255920258 ns/op	504088876 B/op	 7876351 allocs/op
// BenchmarkSearch/difficulty=22_cores=8-12         	      10	 201556788 ns/op	558042440 B/op	 8719356 allocs/op
// BenchmarkSearch/difficulty=24_cores=1-12         	      10	2141673050 ns/op	1294475182 B/op	20226149 allocs/op
// BenchmarkSearch/difficulty=24_cores=2-12         	      10	1759634617 ns/op	2028179908 B/op	31690253 allocs/op
// BenchmarkSearch/difficulty=24_cores=4-12         	      10	 981203300 ns/op	1917939096 B/op	29967719 allocs/op
// BenchmarkSearch/difficulty=24_cores=8-12         	      10	1029257296 ns/op	2799724116 B/op	43745459 allocs/op
// BenchmarkSearch/difficulty=26_cores=1-12         	      10	25587392621 ns/op	15294127182 B/op	238970344 allocs/op
// BenchmarkSearch/difficulty=26_cores=2-12         	      10	9736256296 ns/op	11060180190 B/op	172814962 allocs/op
// BenchmarkSearch/difficulty=26_cores=4-12         	      10	7760700688 ns/op	14721375220 B/op	230020906 allocs/op
// BenchmarkSearch/difficulty=26_cores=8-12         	      10	2061354667 ns/op	5178224786 B/op	80909309 allocs/op
// BenchmarkSearch/difficulty=28_cores=1-12         	      10	72567312888 ns/op	43706653524 B/op	682915321 allocs/op
// BenchmarkSearch/difficulty=28_cores=2-12         	      10	16132176175 ns/op	18316828398 B/op	286199839 allocs/op
// BenchmarkSearch/difficulty=28_cores=4-12         	      10	16948325900 ns/op	32005616084 B/op	500086387 allocs/op

func BenchmarkSearch(b *testing.B) {
	salt, err := New()
	if err != nil {
		b.Fatal(err)
	}
	for _, difficulty := range []uint16{22, 24, 26, 28} {
		for _, cores := range []int{1, 2, 4, 8} {
			b.Run(fmt.Sprintf("difficulty=%d cores=%d", difficulty, cores), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					Search(salt, difficulty, cores)
				}
			})
		}
	}
}
