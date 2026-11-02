// Copyright 2026 Anas Ait Said Oubrahim

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

// Benchmarks for the scan and the duplicate detection on a synthetic tree.
// Run them with:
//
//	go test -run '^$' -bench . -benchmem

import (
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// treeSpec describes a synthetic directory tree: small files that come in a
// few sizes with every second one a copy of the previous one, large files
// that all share a size but differ from their first byte, and pairs of
// identical large files.
type treeSpec struct {
	smallFiles int
	largeFiles int
	largeSize  int
	largePairs int
}

// benchSpec is the tree the benchmarks run on. All of its files share a
// size with another, so each one is a candidate that must be looked at.
var benchSpec = treeSpec{smallFiles: 2000, largeFiles: 100, largeSize: 1 << 20, largePairs: 20}

// buildTree writes spec into a new temporary directory and returns it along
// with the total size of the files written.
func buildTree(tb testing.TB, spec treeSpec) (dir string, totalBytes int64) {
	tb.Helper()
	dir = tb.TempDir()
	src := rand.NewChaCha8([32]byte{'d', 'u', 'p'})
	write := func(name string, content []byte) {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			tb.Fatal(err)
		}
		totalBytes += int64(len(content))
	}

	var prev []byte
	for i := range spec.smallFiles {
		content := prev
		if i%2 == 0 {
			content = make([]byte, 512*(1+i%8))
			src.Read(content)
		}
		write(fmt.Sprintf("small/%02d/f%05d", i%50, i), content)
		prev = content
	}
	for i := range spec.largeFiles {
		content := make([]byte, spec.largeSize)
		src.Read(content)
		write(fmt.Sprintf("large/%02d/f%04d", i%20, i), content)
	}
	for i := range spec.largePairs {
		content := make([]byte, spec.largeSize)
		src.Read(content)
		write(fmt.Sprintf("pairs/a/f%04d", i), content)
		write(fmt.Sprintf("pairs/b/f%04d", i), content)
	}
	return dir, totalBytes
}

func BenchmarkScan(b *testing.B) {
	dir, _ := buildTree(b, benchSpec)
	b.ResetTimer()
	for range b.N {
		if _, _, err := scan([]string{dir}, false, io.Discard); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFindDuplicates(b *testing.B) {
	benchmarkFindDuplicates(b, runtime.NumCPU())
}

func BenchmarkFindDuplicatesOneWorker(b *testing.B) {
	benchmarkFindDuplicates(b, 1)
}

func benchmarkFindDuplicates(b *testing.B, workers int) {
	dir, total := buildTree(b, benchSpec)
	groups, _, err := scan([]string{dir}, false, io.Discard)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(total)
	b.ResetTimer()
	for range b.N {
		if _, errs := findDuplicates(groups, workers, io.Discard); errs != 0 {
			b.Fatalf("%d files could not be hashed", errs)
		}
	}
}
