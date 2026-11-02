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

// Command dup_files_detector lists the files with identical content found
// under one or more directories and can delete the redundant copies.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// stringList collects the values of a repeatable flag.
type stringList []string

func (l *stringList) String() string     { return strings.Join(*l, ", ") }
func (l *stringList) Set(v string) error { *l = append(*l, v); return nil }

// run is the whole program. It returns the exit code: 0 on success, 1 when
// the user declines to continue, the scan fails or something had to be
// skipped (the results may be incomplete), 2 on a usage error.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("dup_files_detector", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: dup_files_detector [flags]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Lists the files with identical content found under the given directories.")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Flags:")
		flags.PrintDefaults()
	}
	var dirs stringList
	flags.Var(&dirs, "directoryPath", "Directory to check; repeat the flag to check several (default \".\").")
	ignoreEmpty := flags.Bool("ignoreEmpty", false, "Ignore empty files.")
	deleteDups := flags.Bool("deleteDuplicates", false, "Delete found duplicates (use with caution!).")
	showVersion := flags.Bool("version", false, "Print the version and exit.")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument %q: directories are given with --directoryPath\n", flags.Arg(0))
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, "dup_files_detector", version())
		return 0
	}
	if len(dirs) == 0 {
		dirs = stringList{"."}
	}

	if *deleteDups {
		fmt.Fprint(stdout, "WARNING: deleting duplicated files is enabled. Do you want to continue? (y/N) ")
		if !confirm(stdin) {
			return 1
		}
	}

	fmt.Fprintf(stdout, "Processing files in the following directory(ies): %v\n", []string(dirs))

	groups, stats, err := scan(dirs, *ignoreEmpty, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	dups, hashErrors := findDuplicates(groups, stderr)

	for _, g := range dups {
		fmt.Fprintf(stdout, "%s:\n", g.hash[:10])
		for i, path := range g.paths {
			fmt.Fprintf(stdout, "\t%s", path)
			if *deleteDups && i > 0 {
				fmt.Fprint(stdout, " ... Deleting duplicate.")
				if err := os.Remove(path); err != nil {
					panic(err)
				}
			}
			fmt.Fprintln(stdout)
		}
	}

	if problems := stats.errors + hashErrors; problems > 0 {
		fmt.Fprintf(stderr, "%d problem(s) reported above; the results may be incomplete.\n", problems)
		return 1
	}
	return 0
}

// confirm reads one line from r and reports whether it is a "yes".
func confirm(r io.Reader) bool {
	line, _ := bufio.NewReader(r).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}

// version is the module version when the binary was built with go install,
// and "(devel)" for local builds.
func version() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "unknown"
}
