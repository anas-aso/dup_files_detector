// Copyright 2020 Anas Ait Said Oubrahim

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
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/alecthomas/kingpin.v2"
)

// for a give file path, return the sha265 hash of the file's content
func fileToHash(filePath string) (hash string, err error) {
	hasher := sha256.New()

	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	_, err = io.Copy(hasher, file)
	if err != nil {
		return
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// map of files paths' with identical hash value (key)
type identicalFiles map[string][]string

func main() {
	directoriesPaths := kingpin.Flag("directoryPath", "Path to the directory(ies) you want to check (repeatable).").Default("./").Strings()
	kingpin.Version("Duplicated files detector : 0.0.1")
	kingpin.Parse()

	fmt.Printf("Processing files in the following directory(ies): %v\n", *directoriesPaths)

	result := make(identicalFiles)

	// iterate over provided directories
	for _, dir := range *directoriesPaths {
		err := filepath.Walk(dir,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				// ignore directories and symlinks
				if info.Mode().IsRegular() {
					h, err := fileToHash(path)
					if err != nil {
						panic(err)
					}

					if _, ok := result[h]; ok {
						result[h] = append(result[h], path)
					} else {
						result[h] = []string{path}
					}
				}
				return nil
			})

		if err != nil {
			panic(err)
		}
	}

	for k, v := range result {
		if len(v) > 1 {
			fmt.Printf("%v:\n", k)
			for _, file := range v {
				fmt.Printf("\t%v\n", file)
			}
		}
	}

}
