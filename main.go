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
	"runtime"
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
// something failed or had to be skipped (the results may be incomplete), and
// 2 on a usage error. Progress and warnings go to stderr, the report to
// stdout.
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
	deleteDups := flags.Bool("deleteDuplicates", false, "Delete the duplicates, keeping the first file of each group (asks for confirmation).")
	workers := flags.Int("workers", runtime.NumCPU(), "Number of files to hash concurrently.")
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
	if *workers < 1 {
		fmt.Fprintln(stderr, "--workers must be at least 1")
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
		fmt.Fprint(stderr, "WARNING: deleting duplicated files is enabled. Do you want to continue? (y/N) ")
		if !confirm(stdin) {
			fmt.Fprintln(stderr, "Aborted.")
			return 1
		}
	}

	fmt.Fprintf(stderr, "Scanning %s ...\n", strings.Join(dirs, ", "))
	groups, stats, err := scan(dirs, *ignoreEmpty, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	candidates, candidateBytes := 0, int64(0)
	for _, g := range groups {
		candidates += len(g.paths)
		candidateBytes += g.size * int64(len(g.paths))
	}
	fmt.Fprintf(stderr, "Scanned %s; comparing the %s that share a size with another (%s) ...\n",
		plural(stats.files, "file"), plural(candidates, "file"), formatBytes(candidateBytes))

	dups, hashErrors := findDuplicates(groups, *workers, stderr)
	rs := report(stdout, dups, *deleteDups)
	if len(dups) == 0 {
		fmt.Fprintln(stdout, "No duplicates found.")
	} else {
		fmt.Fprintf(stdout, "\nFound %s of duplicates: %s, %s reclaimable.\n",
			plural(len(dups), "group"), plural(rs.redundant, "redundant file"), formatBytes(rs.reclaimable))
		if *deleteDups {
			fmt.Fprintf(stdout, "Deleted %s, freed %s.\n", plural(rs.deleted, "file"), formatBytes(rs.freed))
		}
	}

	if problems := stats.errors + hashErrors + rs.failed; problems > 0 {
		fmt.Fprintf(stderr, "%s reported above; the results may be incomplete.\n", plural(problems, "problem"))
		return 1
	}
	return 0
}

// reportStats summarises what report printed and did.
type reportStats struct {
	redundant   int   // files beyond the first one of each group
	reclaimable int64 // bytes taken up by the redundant files
	deleted     int
	freed       int64
	failed      int // deletions that failed
}

// report prints the duplicate groups to w: a line per group followed by its
// paths, one per indented line. With del set it also deletes every file of
// each group except the first one, noting the outcome next to the path.
func report(w io.Writer, groups []fileGroup, del bool) reportStats {
	var rs reportStats
	for _, g := range groups {
		fmt.Fprintf(w, "%s: %s, %s each\n", g.hash[:10], plural(len(g.paths), "file"), formatBytes(g.size))
		for i, path := range g.paths {
			fmt.Fprintf(w, "\t%s", path)
			if i > 0 {
				rs.redundant++
				rs.reclaimable += g.size
				if del {
					if err := deleteDuplicate(g.paths[0], path, g.size); err != nil {
						rs.failed++
						fmt.Fprintf(w, " ... NOT deleted: %v", err)
					} else {
						rs.deleted++
						rs.freed += g.size
						fmt.Fprint(w, " ... deleted")
					}
				}
			}
			fmt.Fprintln(w)
		}
	}
	return rs
}

// deleteDuplicate removes dup, but only after checking that both it and
// kept, the copy that stays, still are the regular files of the given size
// that the scan saw. A file that changed or vanished in the meantime, or
// whose kept copy did, is left alone.
func deleteDuplicate(kept, dup string, size int64) error {
	if err := checkUnchanged(kept, size); err != nil {
		return fmt.Errorf("the copy being kept (%s) %w", kept, err)
	}
	if err := checkUnchanged(dup, size); err != nil {
		return fmt.Errorf("the file %w", err)
	}
	if err := os.Remove(dup); err != nil {
		return errors.New(errText(err))
	}
	return nil
}

func checkUnchanged(path string, size int64) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("cannot be checked: %s", errText(err))
	}
	if !info.Mode().IsRegular() {
		return errors.New("is no longer a regular file")
	}
	if info.Size() != size {
		return errors.New("changed size since it was scanned")
	}
	return nil
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

// plural renders a count with its noun: "1 file", "2 files".
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// formatBytes renders a size in binary units: "512 B", "1.5 MiB".
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
