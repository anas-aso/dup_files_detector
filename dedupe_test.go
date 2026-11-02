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
	"os"
	"path/filepath"
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

func TestRunUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"--bogus"}, {"some/dir"}} {
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
