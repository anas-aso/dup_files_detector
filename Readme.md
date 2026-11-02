# Duplicated files detector

A command line tool that finds files with identical content in one or more
directories and, if asked to, deletes the redundant copies.

I originally built it to clean up duplicated pictures on my hard drive, but it
works for any kind of file.

## Install

Requires [Go](https://go.dev/doc/install) 1.27 or newer.

```bash
go install github.com/anas-aso/dup_files_detector@latest
```

## Build

```bash
go build -ldflags="-s -w" -o dup_files_detector .
```

## Usage

Check the current directory:

```bash
./dup_files_detector
```

Check one or more directories:

```bash
./dup_files_detector --directoryPath=path/to/dir1 --directoryPath=path/to/dir2
```

Delete the duplicates, keeping the first copy of each group (asks for
confirmation first):

```bash
./dup_files_detector --directoryPath=path/to/dir --deleteDuplicates
```

| Flag                 | Description                                                                                         |
| -------------------- | --------------------------------------------------------------------------------------------------- |
| `--directoryPath`    | Directory to check. Repeat the flag to check several. Defaults to `.`.                              |
| `--ignoreEmpty`      | Ignore empty files. Otherwise all empty files form one group of duplicates.                         |
| `--deleteDuplicates` | Delete every file of a group except the first one. Asks for confirmation before scanning.           |
| `--workers`          | Number of files hashed concurrently. Defaults to the number of CPUs; use `1` on a spinning disk.    |
| `--version`          | Print the version and exit.                                                                         |

The report goes to stdout, progress and warnings to stderr, so
`./dup_files_detector > report.txt` keeps just the report:

```
e3b0c44298: 2 files, 3.2 MiB each
	photos/2019/IMG_0042.jpg
	backup/photos/IMG_0042.jpg
6f0dc6b1a3: 3 files, 48.0 KiB each
	docs/notes.pdf
	docs/old/notes.pdf
	downloads/notes (1).pdf

Found 2 groups of duplicates: 3 redundant files, 3.3 MiB reclaimable.
```

Exit codes: `0` when everything went well, `1` when something failed or had to
be skipped (the results may be incomplete), `2` on a usage error.

## How it works

1. The directories are walked and regular files are grouped by size. A file
   whose size is unique cannot have a duplicate, so it is never read.
2. The first 16 KiB of each remaining file is hashed (SHA-256). This cheaply
   separates files that merely share a size.
3. The files still matching are hashed in full, several at a time.

Things worth knowing:

- **Which copy is kept.** The first file found. Directories are scanned in the
  order given, each in lexical order, so put the directory whose copies you
  want to keep first.
- **Symbolic links** are never followed, with one exception: a directory given
  on the command line may itself be a symbolic link.
- **Hard links** to the same file, and a file reached through overlapping
  directories, count as one file, not as duplicates.
- **Unreadable** files or directories are reported on stderr and skipped; the
  scan carries on.
- **Before a file is deleted**, both it and the copy being kept are checked to
  still be regular files of the size that was scanned.

## Development

```bash
go test ./...                              # unit tests
go test -run '^$' -bench . -benchmem ./... # benchmarks
```

## License

[Apache 2.0](LICENSE)
