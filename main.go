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
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

// map of files paths' with identical size
type identicalSizes map[int64][]string

func main() {
	directoriesPaths := kingpin.Flag("directoryPath", "Path to the directory(ies) you want to check (repeatable).").Default("./").Strings()
	ignoreEmpty := kingpin.Flag("ignoreEmpty", "Ignore empty files.").Default("false").Bool()
	deleteDuplicates := kingpin.Flag("deleteDuplicates", "Delete found duplicates (use with caution!).").Default("false").Bool()
	kingpin.Version("Duplicated files detector : 0.0.1")
	kingpin.Parse()

	if *deleteDuplicates {
		fmt.Printf("WARNING: deleting duplicated files is enabled. Do you want to continue ? (y/N) ")
		response, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		response = strings.TrimSuffix(response, "\n")
		if response != "y" {
			os.Exit(1)
		}
	}

	fmt.Printf("Processing files in the following directory(ies): %v\n", *directoriesPaths)

	// Groups files with the same size.
	// This optimization is needed to avoid calculating checksum for
	// files without duplicates which is expensive for large files.
	filesWithSameSize := make(identicalSizes)
	for _, dir := range *directoriesPaths {
		err := filepath.Walk(dir,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				// ignore directories and symlinks
				if info.Mode().IsRegular() {
					if *ignoreEmpty && info.Size() == 0 {
						return nil
					}

					if _, ok := filesWithSameSize[info.Size()]; ok {
						filesWithSameSize[info.Size()] = append(filesWithSameSize[info.Size()], path)
					} else {
						filesWithSameSize[info.Size()] = []string{path}
					}
				}
				return nil
			})

		if err != nil {
			panic(err)
		}
	}

	// evaluate hash for the collected files
	result := make(identicalFiles)
	for _, group := range filesWithSameSize {
		if len(group) > 1 {
			for _, path := range group {
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
		}
	}

	// print files with identical content
	for k, v := range result {
		if len(v) > 1 {
			fmt.Printf("%v:\n", k[:10])
			for idx, file := range v {
				fmt.Printf("\t%v", file)

				// if requested, delete duplicated files.
				if *deleteDuplicates && idx != 0 {
					fmt.Printf(" ... Deleting duplicate.")
					err := os.Remove(file)
					if err != nil {
						panic(err)
					}
				}
				fmt.Println()
			}
		}
	}

}
