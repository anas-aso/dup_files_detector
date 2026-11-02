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

//go:build !unix

package main

import "io/fs"

// identify returns the identity of the file described by info. Without
// device and inode numbers the absolute path has to do, so on this platform
// hard links are not recognised as being the same file.
func identify(path string, _ fs.FileInfo) fileID {
	return fileID{path: path}
}
