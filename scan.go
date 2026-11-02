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
	"cmp"
	"io/fs"
	"path/filepath"
	"slices"
)

// fileGroup is a set of files that share the same size and, once hashed, the
// same content. paths are in discovery order: the first one is the copy that
// is kept when duplicates are deleted.
type fileGroup struct {
	size  int64
	hash  string // hex SHA-256 of the content, set once the group has been hashed
	paths []string
}

// scan walks dirs and returns the groups of regular files that share a size,
// which are the only candidates for being duplicates. Directories are walked
// in the order given, each in lexical order. Symbolic links are not followed.
// Groups are returned sorted by size, largest first, then by first path.
func scan(dirs []string, ignoreEmpty bool) ([]fileGroup, error) {
	bySize := make(map[int64][]string)
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.Type().IsRegular() {
				return nil // directories, symlinks, devices, ...
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			size := info.Size()
			if ignoreEmpty && size == 0 {
				return nil
			}
			bySize[size] = append(bySize[size], path)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	var groups []fileGroup
	for size, paths := range bySize {
		if len(paths) > 1 {
			groups = append(groups, fileGroup{size: size, paths: paths})
		}
	}
	sortGroups(groups)
	return groups, nil
}

// sortGroups orders groups by size, largest first, then by first path, so
// that the output is deterministic.
func sortGroups(groups []fileGroup) {
	slices.SortFunc(groups, func(a, b fileGroup) int {
		if c := cmp.Compare(b.size, a.size); c != 0 {
			return c
		}
		return cmp.Compare(a.paths[0], b.paths[0])
	})
}
