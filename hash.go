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
	"io"
	"os"
)

// hashFile returns the hex-encoded SHA-256 of the content of the file.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// findDuplicates narrows groups of same-size files down to groups of files
// with identical content by splitting each group by the hash of its members.
// Groups left with a single member are dropped; the order of the groups and
// of the members of a group is preserved.
func findDuplicates(groups []fileGroup) ([]fileGroup, error) {
	var out []fileGroup
	for _, g := range groups {
		var subgroups []fileGroup
		byHash := make(map[string]int) // hash -> index in subgroups
		for _, path := range g.paths {
			hash, err := hashFile(path)
			if err != nil {
				return nil, err
			}
			i, ok := byHash[hash]
			if !ok {
				i = len(subgroups)
				byHash[hash] = i
				subgroups = append(subgroups, fileGroup{size: g.size, hash: hash})
			}
			subgroups[i].paths = append(subgroups[i].paths, path)
		}
		for _, sub := range subgroups {
			if len(sub.paths) > 1 {
				out = append(out, sub)
			}
		}
	}
	return out, nil
}
