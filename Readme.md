# Duplicated files detector

A helper application to detect and list duplicated files in a directory.

I originally built it to cleanup duplicated pictures on my hard drive. But it works for other files types as well.

## Build
```bash
go build -a -installsuffix cgo -ldflags="-w -s" -o dup_files_detector
```

## Usage
Process a single directory
```bash
./dup_files_detector --root.path=path/to/check
```

Or, process multiple directories
```bash
./dup_files_detector --directoryPath=path/to/dir1 --directoryPath=path/to/dir2 ...
```
