# Duplicated files detector

A helper CLI to detect and list duplicated files in a directory.

I originally built it to cleanup duplicated pictures on my hard drive. But it works for other files types as well.

## Build
Requirement: [Rust](https://doc.rust-lang.org/book/ch01-01-installation.html)

```bash
cargo build --release
```

## Usage
Process a single directory
```bash
./target/release/dup_files_detector --directoryPath=path/to/check
```

Or, process multiple directories
```bash
./target/release/dup_files_detector --directoryPath=path/to/dir1 --directoryPath=path/to/dir2 ...
```
