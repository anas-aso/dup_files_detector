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
	"errors"
	"fmt"
	"io"
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

// scanStats summarises a scan.
type scanStats struct {
	files  int // regular files recorded
	errors int // entries that could not be read and were skipped
}

// scan walks dirs and returns the groups of regular files that share a size,
// which are the only candidates for being duplicates. Directories are walked
// in the order given, each in lexical order. Symbolic links are not followed.
// Entries that cannot be read are reported on warnings and skipped. Groups
// are returned sorted by size, largest first, then by first path.
func scan(dirs []string, ignoreEmpty bool, warnings io.Writer) ([]fileGroup, scanStats, error) {
	var stats scanStats
	bySize := make(map[int64][]string)
	skip := func(path string, err error) {
		stats.errors++
		fmt.Fprintf(warnings, "warning: skipping %s: %s\n", path, errText(err))
	}

	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				// Usually a directory we may not read: report it and carry on
				// with the rest of the tree.
				skip(path, err)
				return nil
			}
			if !d.Type().IsRegular() {
				return nil // directories, symlinks, devices, ...
			}
			info, err := d.Info()
			if err != nil {
				skip(path, err)
				return nil
			}
			size := info.Size()
			if ignoreEmpty && size == 0 {
				return nil
			}
			stats.files++
			bySize[size] = append(bySize[size], path)
			return nil
		})
		if err != nil {
			return nil, stats, err
		}
	}

	var groups []fileGroup
	for size, paths := range bySize {
		if len(paths) > 1 {
			groups = append(groups, fileGroup{size: size, paths: paths})
		}
	}
	sortGroups(groups)
	return groups, stats, nil
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

// errText describes err without the path of a *fs.PathError, which callers
// print themselves.
func errText(err error) string {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err.Error()
	}
	return err.Error()
}
