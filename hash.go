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

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sync"
)

// prefixSize is how much of each file the first hashing pass reads. Files
// that merely share a size usually differ within their first few kilobytes,
// so this pass rules most of them out without reading them in full.
const prefixSize = 16 << 10

// readBufferSize is the size of the buffer each worker hashes through.
const readBufferSize = 1 << 20

// hashFile returns the hex-encoded SHA-256 of the first limit bytes of the
// file (of all of it if it is shorter) and the number of bytes read.
func hashFile(path string, limit int64, buf []byte) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	h := sha256.New()
	n, err := io.CopyBuffer(h, io.LimitReader(f, limit), buf)
	if err != nil {
		return "", n, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// findDuplicates narrows groups of same-size files down to groups of files
// with identical content, and returns them sorted like scan does. It hashes
// in two passes: first a short prefix of every file, which cheaply splits
// groups of files that only share a size, then the full content of the files
// still in a group. Large files are therefore only read in full when they
// are likely duplicates. The number of files that could not be read is
// returned as well.
func findDuplicates(groups []fileGroup, workers int, warnings io.Writer) ([]fileGroup, int) {
	groups, errs := hashGroups(groups, prefixSize, workers, warnings)

	// Files no larger than the prefix have already been hashed in full.
	var done, pending []fileGroup
	for _, g := range groups {
		if g.size <= prefixSize {
			done = append(done, g)
		} else {
			pending = append(pending, g)
		}
	}
	full, moreErrs := hashGroups(pending, math.MaxInt64, workers, warnings)

	result := append(done, full...)
	sortGroups(result)
	return result, errs + moreErrs
}

// hashGroups hashes the first limit bytes of every file of every group, using
// the given number of concurrent workers, and splits each group by hash.
// Groups left with a single member are dropped; the order of the members of
// a group is preserved. A file that cannot be read, or that no longer has the
// size recorded by the scan, is reported on warnings and dropped.
func hashGroups(groups []fileGroup, limit int64, workers int, warnings io.Writer) ([]fileGroup, int) {
	type job struct{ group, member int }
	type result struct {
		hash string
		err  error
	}

	results := make([][]result, len(groups))
	for i, g := range groups {
		results[i] = make([]result, len(g.paths))
	}

	jobs := make(chan job)
	var wg sync.WaitGroup
	for range max(workers, 1) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, readBufferSize)
			for j := range jobs {
				g := groups[j.group]
				hash, n, err := hashFile(g.paths[j.member], limit, buf)
				if err == nil && n != min(g.size, limit) {
					err = errors.New("size changed since it was scanned")
				}
				results[j.group][j.member] = result{hash: hash, err: err}
			}
		}()
	}
	for gi, g := range groups {
		for mi := range g.paths {
			jobs <- job{group: gi, member: mi}
		}
	}
	close(jobs)
	wg.Wait()

	var out []fileGroup
	errs := 0
	for gi, g := range groups {
		var subgroups []fileGroup
		byHash := make(map[string]int) // hash -> index in subgroups
		for mi, path := range g.paths {
			r := results[gi][mi]
			if r.err != nil {
				errs++
				fmt.Fprintf(warnings, "warning: skipping %s: %s\n", path, errText(r.err))
				continue
			}
			i, ok := byHash[r.hash]
			if !ok {
				i = len(subgroups)
				byHash[r.hash] = i
				subgroups = append(subgroups, fileGroup{size: g.size, hash: r.hash})
			}
			subgroups[i].paths = append(subgroups[i].paths, path)
		}
		for _, sub := range subgroups {
			if len(sub.paths) > 1 {
				out = append(out, sub)
			}
		}
	}
	return out, errs
}
