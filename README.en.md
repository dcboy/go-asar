# go-asar (English)

A Go implementation of the ASAR pack/unpack library and CLI aligned with Electron’s `asar` (node-asar).

- Pack any app directory to `.asar`
- Extract `.asar` to a destination
- Read and parse ASAR header (Pickle + JSON)
- List all paths in the archive
- Compute file integrity (whole file hash and 4MB block hashes, `SHA256`)

---

## Install

Library:

```
go get github.com/dcboy/go-asar
```

CLI:

```
go build -o ./bin/go-asar ./cmd/asar
```

---

## Quick Start

Pack:

```
./bin/go-asar pack ./path/to/app ./app.asar
```

Extract:

```
./bin/go-asar extract ./app.asar ./unpacked
```

List:

```
files, _ := asar.ListPackage("./app.asar", false)
for _, f := range files { fmt.Println(f) }
```

---

## CLI Commands & Options

- pack
  - Syntax: `asar pack <dir> <output> [--ordering <file>] [--unpack <glob>] [--unpack-dir <glob|prefix>] [--exclude-hidden]`
  - Notes:
    - `--ordering <file>` specifies insertion order file (one path per line; supports `a:b` prefix format), aligned with node-asar
    - `--unpack <glob>` matches files to be copied to `<output>.unpacked` instead of packing
    - `--unpack-dir <glob|prefix>` matches directories (glob or prefix) to be unpacked to `<output>.unpacked`
    - `--exclude-hidden` excludes hidden files (any path segment starting with `.`)
  - Examples:
    - `./bin/go-asar pack ./app ./app.asar`
    - `./bin/go-asar pack ./app ./app.asar --exclude-hidden`
    - `./bin/go-asar pack ./app ./app.asar --ordering ./ordering.txt`
    - `./bin/go-asar pack ./app ./app.asar --unpack "*.png"`
    - `./bin/go-asar pack ./app ./app.asar --unpack-dir "assets/**"`

- list
  - Syntax: `asar list <archive> [-i | --is-pack]`
  - Notes: when `--is-pack` is set, each path is prefixed with `pack   :` or `unpack :`
  - Examples:
    - `./bin/go-asar list ./app.asar`
    - `./bin/go-asar list ./app.asar --is-pack`

- extract-file
  - Syntax: `asar extract-file <archive> <filename>`
  - Notes: extracts a single file to `basename(filename)` in current directory
  - Example:
    - `./bin/go-asar extract-file ./app.asar dir1/file1.txt`

- extract
  - Syntax: `asar extract <archive> <dest>`
  - Notes: extracts the whole archive to destination
  - Example:
    - `./bin/go-asar extract ./app.asar ./unpacked`

---

## Go API

- `CreatePackage(src, dest string) error`
- `CreatePackageWithOptions(src, dest string, options CreateOptions) error`
- `ExtractAll(archivePath, dest string) error`
- `ExtractFile(archivePath, filename string, followLinks bool) ([]byte, error)`
- `ListPackage(archivePath string, isPack bool) ([]string, error)`
- `GetRawHeader(archivePath string) (ArchiveHeader, error)`

---

## Design

- Header format: size-pickle (payload length) + header-pickle (JSON string), followed by file contents in order
- Integrity: `SHA256` for whole file and 4MB blocks
- Safety: path traversal checks when extracting; symlink target validation
- Relative paths: normalized to archive root; symlinks handled with string prefix trimming then `filepath.Rel` fallback

---

## License

- MIT

---

## Acknowledgements

- Electron `asar` and the `node-asar` reference implementation
