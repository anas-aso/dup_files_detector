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
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

// bigContent returns more than prefixSize bytes ending in last, so that two
// results with different last bytes share a prefix but differ as a whole.
func bigContent(last byte) []byte {
	b := make([]byte, prefixSize+4096)
	for i := range b {
		b[i] = byte(i % 251)
	}
	b[len(b)-1] = last
	return b
}

// dupsIn scans dirs and returns the duplicate groups found and the number of
// files that could not be hashed.
func dupsIn(t *testing.T, ignoreEmpty bool, dirs ...string) ([]fileGroup, int) {
	t.Helper()
	groups, _, err := scan(dirs, ignoreEmpty, io.Discard)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	return findDuplicates(groups, 3, io.Discard)
}

func assertGroups(t *testing.T, got []fileGroup, want [][]string) {
	t.Helper()
	gotPaths := make([][]string, len(got))
	for i, g := range got {
		gotPaths[i] = g.paths
	}
	if !slices.EqualFunc(gotPaths, want, slices.Equal) {
		t.Errorf("duplicate groups:\n got %q\nwant %q", gotPaths, want)
	}
}

func assertExists(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if _, err := os.Lstat(p); err != nil {
			t.Errorf("%s should still exist: %v", p, err)
		}
	}
}

func assertGone(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Errorf("%s should have been deleted (lstat error: %v)", path, err)
	}
}

func skipIfPermissionsAreNotEnforced(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("file permissions work differently on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root, permissions are not enforced")
	}
}

func TestFindsFilesWithIdenticalContent(t *testing.T) {
	dir := t.TempDir()
	x := filepath.Join(dir, "a", "x.txt")
	y := filepath.Join(dir, "b", "y.txt")
	writeFile(t, x, []byte("hello"))
	writeFile(t, y, []byte("hello"))
	writeFile(t, filepath.Join(dir, "b", "z.txt"), []byte("world"))  // same size, other content
	writeFile(t, filepath.Join(dir, "c", "w.txt"), []byte("hello!")) // unique size

	dups, errs := dupsIn(t, false, dir)
	if errs != 0 {
		t.Errorf("errors = %d, want 0", errs)
	}
	assertGroups(t, dups, [][]string{{x, y}})
	if len(dups) == 1 {
		if dups[0].size != 5 {
			t.Errorf("size = %d, want 5", dups[0].size)
		}
		if want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"; dups[0].hash != want {
			t.Errorf("hash = %s, want the SHA-256 of \"hello\" (%s)", dups[0].hash, want)
		}
	}
}

func TestLargeFilesAreComparedInFull(t *testing.T) {
	dir := t.TempDir()
	a, b, c := filepath.Join(dir, "a"), filepath.Join(dir, "b"), filepath.Join(dir, "c")
	writeFile(t, a, bigContent(1))
	writeFile(t, b, bigContent(1))
	writeFile(t, c, bigContent(2)) // same size and prefix, different tail

	dups, _ := dupsIn(t, false, dir)
	assertGroups(t, dups, [][]string{{a, b}})
}

func TestEmptyFiles(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	writeFile(t, a, nil)
	writeFile(t, b, nil)

	dups, _ := dupsIn(t, false, dir)
	assertGroups(t, dups, [][]string{{a, b}})
	dups, _ = dupsIn(t, true, dir)
	assertGroups(t, dups, nil)
}

func TestGroupsAreOrderedBySizeThenPath(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"b/1": "aa", "b/2": "aa",
		"a/1": "bb", "a/2": "bb",
		"c/1": "ccc", "c/2": "ccc",
	} {
		writeFile(t, filepath.Join(dir, name), []byte(content))
	}
	p := func(name string) string { return filepath.Join(dir, name) }

	dups, _ := dupsIn(t, false, dir)
	assertGroups(t, dups, [][]string{
		{p("c/1"), p("c/2")}, // largest first
		{p("a/1"), p("a/2")}, // then by first path
		{p("b/1"), p("b/2")},
	})
}

func TestOverlappingRootsDoNotPairAFileWithItself(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	writeFile(t, filepath.Join(sub, "only"), []byte("one copy"))

	for _, roots := range [][]string{{dir, dir}, {dir, sub}, {sub, dir}} {
		dups, _ := dupsIn(t, false, roots...)
		assertGroups(t, dups, nil)
	}
}

func TestHardLinksCountAsOneFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hard links are not recognised on Windows")
	}
	dir := t.TempDir()
	a, c := filepath.Join(dir, "a"), filepath.Join(dir, "c")
	writeFile(t, a, []byte("linked"))
	if err := os.Link(a, filepath.Join(dir, "b")); err != nil {
		t.Skipf("hard links not supported here: %v", err)
	}
	writeFile(t, c, []byte("linked")) // a real copy

	dups, _ := dupsIn(t, false, dir)
	assertGroups(t, dups, [][]string{{a, c}})
}

func TestSymlinks(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	writeFile(t, filepath.Join(real, "a"), []byte("content"))
	writeFile(t, filepath.Join(real, "b"), []byte("content"))
	if err := os.Symlink(filepath.Join(real, "a"), filepath.Join(real, "link-to-a")); err != nil {
		t.Skipf("symlinks not supported here: %v", err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	// Symlinked files are ignored, a root that is a symlink is followed, and
	// paths are reported under the name the root was given as.
	dups, _ := dupsIn(t, false, link)
	assertGroups(t, dups, [][]string{{filepath.Join(link, "a"), filepath.Join(link, "b")}})
}

func TestRelativeRootGivesRelativePaths(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a"), []byte("x"))
	writeFile(t, filepath.Join(dir, "sub", "b"), []byte("x"))
	t.Chdir(dir)

	for _, root := range []string{".", "./", "sub/.."} {
		dups, _ := dupsIn(t, false, root)
		assertGroups(t, dups, [][]string{{"a", filepath.Join("sub", "b")}})
	}
}

func TestScanRejectsBadRoots(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	writeFile(t, file, []byte("x"))

	for _, root := range []string{filepath.Join(dir, "missing"), file} {
		if _, _, err := scan([]string{root}, false, io.Discard); err == nil {
			t.Errorf("scan(%s) succeeded, want an error", root)
		}
	}
}

func TestUnreadableDirectoryIsSkipped(t *testing.T) {
	skipIfPermissionsAreNotEnforced(t)
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	writeFile(t, a, []byte("dup"))
	writeFile(t, b, []byte("dup"))
	locked := filepath.Join(dir, "locked")
	writeFile(t, filepath.Join(locked, "c"), []byte("dup"))
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })

	var warnings bytes.Buffer
	groups, stats, err := scan([]string{dir}, false, &warnings)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if stats.errors != 1 || stats.files != 2 {
		t.Errorf("stats = %+v, want 1 error and 2 files", stats)
	}
	if !strings.Contains(warnings.String(), locked) {
		t.Errorf("warnings = %q, want a mention of %s", warnings.String(), locked)
	}
	dups, _ := findDuplicates(groups, 2, io.Discard)
	assertGroups(t, dups, [][]string{{a, b}})
}

func TestUnreadableFileIsSkipped(t *testing.T) {
	skipIfPermissionsAreNotEnforced(t)
	dir := t.TempDir()
	a, b, c := filepath.Join(dir, "a"), filepath.Join(dir, "b"), filepath.Join(dir, "c")
	writeFile(t, a, []byte("dup"))
	writeFile(t, b, []byte("dup"))
	writeFile(t, c, []byte("dup"))
	if err := os.Chmod(c, 0); err != nil {
		t.Fatal(err)
	}

	groups, _, err := scan([]string{dir}, false, io.Discard)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	var warnings bytes.Buffer
	dups, errs := findDuplicates(groups, 2, &warnings)
	if errs != 1 || !strings.Contains(warnings.String(), c) {
		t.Errorf("errors = %d, warnings = %q; want 1 error mentioning %s", errs, warnings.String(), c)
	}
	assertGroups(t, dups, [][]string{{a, b}})
}

func TestFileChangedAfterScanIsSkipped(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	writeFile(t, a, []byte("same"))
	writeFile(t, b, []byte("same"))

	groups, _, err := scan([]string{dir}, false, io.Discard)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	writeFile(t, b, []byte("same but longer"))

	dups, errs := findDuplicates(groups, 2, io.Discard)
	if errs != 1 {
		t.Errorf("errors = %d, want 1", errs)
	}
	assertGroups(t, dups, nil)
}

func TestRunReportsDuplicates(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	writeFile(t, a, []byte("dup"))
	writeFile(t, b, []byte("dup"))
	writeFile(t, filepath.Join(dir, "c"), []byte("unique"))

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--directoryPath=" + dir}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if want := ":\n\t" + a + "\n\t" + b + "\n"; !strings.Contains(stdout.String(), want) {
		t.Errorf("stdout = %q, want it to contain %q", stdout.String(), want)
	}
	assertExists(t, a, b)
}

func TestRunDeletesDuplicatesWhenConfirmed(t *testing.T) {
	dir := t.TempDir()
	a, b, c := filepath.Join(dir, "a"), filepath.Join(dir, "b"), filepath.Join(dir, "c")
	writeFile(t, a, []byte("dup"))
	writeFile(t, b, []byte("dup"))
	writeFile(t, c, []byte("unique"))

	var stdout, stderr bytes.Buffer
	code := run([]string{"--directoryPath=" + dir, "--deleteDuplicates"}, strings.NewReader("y\n"), &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	assertGone(t, b)
	assertExists(t, a, c)
}

func TestRunDeletesNothingWithoutConfirmation(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	writeFile(t, a, []byte("dup"))
	writeFile(t, b, []byte("dup"))

	for _, answer := range []string{"", "n\n", "yes please\n"} {
		var stdout, stderr bytes.Buffer
		code := run([]string{"--directoryPath=" + dir, "--deleteDuplicates"}, strings.NewReader(answer), &stdout, &stderr)
		if code != 1 {
			t.Errorf("answer %q: exit code = %d, want 1", answer, code)
		}
		assertExists(t, a, b)
	}
}

func TestRunReportsProblemsInExitCode(t *testing.T) {
	skipIfPermissionsAreNotEnforced(t)
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	writeFile(t, a, []byte("dup"))
	writeFile(t, b, []byte("dup"))
	locked := filepath.Join(dir, "locked")
	writeFile(t, filepath.Join(locked, "c"), []byte("dup"))
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--directoryPath=" + dir}, strings.NewReader(""), &stdout, &stderr); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "\t"+a+"\n\t"+b+"\n") {
		t.Errorf("stdout = %q, want the duplicates that could be read", stdout.String())
	}
	if !strings.Contains(stderr.String(), "warning: skipping "+locked) {
		t.Errorf("stderr = %q, want a warning about %s", stderr.String(), locked)
	}
}

func TestRunMissingDirectory(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--directoryPath", filepath.Join(t.TempDir(), "missing")}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "error:") {
		t.Errorf("exit code = %d, stderr = %q; want 1 and an error message", code, stderr.String())
	}
}

func TestRunUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"--bogus"}, {"some/dir"}, {"--workers=0"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, strings.NewReader(""), &stdout, &stderr); code != 2 {
			t.Errorf("run(%q) = %d, want 2", args, code)
		}
	}
}

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--version"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.HasPrefix(stdout.String(), "dup_files_detector ") {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestConfirm(t *testing.T) {
	for input, want := range map[string]bool{
		"y\n": true, "Y\n": true, "yes\n": true, " y \r\n": true,
		"\n": false, "": false, "n\n": false, "yes please\n": false,
	} {
		if got := confirm(strings.NewReader(input)); got != want {
			t.Errorf("confirm(%q) = %v, want %v", input, got, want)
		}
	}
}
