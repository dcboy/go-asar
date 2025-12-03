# go-asar

中文 | [English](README.en.md)

![CI](https://github.com/dcboy/go-asar/actions/workflows/ci.yml/badge.svg)
![Go](https://img.shields.io/badge/Go-1.21-00ADD8?logo=go)
![License](https://img.shields.io/badge/license-MIT-blue)

Go 语言实现的 ASAR 打包/解包库与命令行工具，功能对齐 Electron 官方 `asar`（node-asar）。可用于：

- 打包任意应用目录为 `.asar`
- 解包 `.asar` 到指定目录
- 读取并解析 ASAR 头部（Pickle + JSON）
- 列出归档内的所有路径
- 计算文件完整性（整文件哈希与 4MB 分块哈希，`SHA256`）

---

## 安装

- 作为库使用（Go 模块）：

```
go get github.com/dcboy/go-asar
```

- 构建命令行工具：

```
go build -o ./bin/go-asar ./cmd/asar
```

---

## 快速开始

- 打包目录为 `.asar`：

```
./bin/go-asar pack --src ./path/to/app --dest ./app.asar
```

- 解包 `.asar` 到目录：

```
./bin/go-asar extract --archive ./app.asar --dest ./unpacked
```

- 列出 `.asar` 文件内的路径（库方法）：

```
files, _ := asar.ListPackage("./app.asar", false)
for _, f := range files { fmt.Println(f) }
```

---

## Go API

- `CreatePackage(src, dest string) error`
  - 用默认选项打包 `src` 目录到 `dest.asar`
- `CreatePackageWithOptions(src, dest string, options CreateOptions) error`
  - 支持选项：
    - `Dot`：是否包含隐藏文件（默认包含）
    - `Ordering`：指定插入顺序的文件列表路径
    - `Pattern`：匹配模式（当前实现主要用于兼容行为）
    - `Unpack`：按文件 glob 规则解包到 `dest.asar.unpacked`
    - `UnpackDir`：按目录前缀/简易 glob 规则解包到 `dest.asar.unpacked`
- `ExtractAll(archivePath, dest string) error`
  - 将 `.asar` 全部解包到 `dest`
- `ExtractFile(archivePath, filename string, followLinks bool) ([]byte, error)`
  - 读取归档内单个文件的二进制内容
- `ListPackage(archivePath string, isPack bool) ([]string, error)`
  - 列出所有路径；`isPack=true` 时附带 `pack/unpack` 标记
- `GetRawHeader(archivePath string) (ArchiveHeader, error)`
  - 返回原始 Pickle 头解析结果（包含 JSON 字符串与嵌套目录结构）

示例：

```
package main

import (
    "fmt"
    "github.com/dcboy/go-asar/asar"
)

func main() {
    if err := asar.CreatePackage("./app", "./app.asar"); err != nil {
        panic(err)
    }
    files, _ := asar.ListPackage("./app.asar", false)
    for _, f := range files { fmt.Println(f) }
}
```

---

## CLI 子命令与选项

- pack

  - 语法：`asar pack <dir> <output> [--ordering <file>] [--unpack <glob>] [--unpack-dir <glob|prefix>] [--exclude-hidden]`
  - 说明：
    - `--ordering <file>` 指定插入顺序文件（每行一个路径，支持 `a:b` 前缀格式，行为与 node-asar 对齐）
    - `--unpack <glob>` 匹配到的文件不打包，直接复制到 `<output>.unpacked`
    - `--unpack-dir <glob|prefix>` 匹配到的目录或以该前缀开头的目录不打包，目录内文件复制到 `<output>.unpacked`
    - `--exclude-hidden` 排除隐藏文件（任一路径段首字符为 `.`），与 node-asar 的 `exclude-hidden` 一致
  - 示例：
    - `./bin/go-asar pack ./app ./app.asar`
    - `./bin/go-asar pack ./app ./app.asar --exclude-hidden`
    - `./bin/go-asar pack ./app ./app.asar --ordering ./ordering.txt`
    - `./bin/go-asar pack ./app ./app.asar --unpack "*.png"`
    - `./bin/go-asar pack ./app ./app.asar --unpack-dir "assets/**"`

- list

  - 语法：`asar list <archive> [-i | --is-pack]`
  - 说明：开启 `--is-pack` 时，在每个路径前输出 `pack   :` 或 `unpack :` 标记
  - 示例：
    - `./bin/go-asar list ./app.asar`
    - `./bin/go-asar list ./app.asar --is-pack`

- extract-file

  - 语法：`asar extract-file <archive> <filename>`
  - 说明：提取单个文件到当前目录的 `basename(filename)`，与 node-asar 行为一致
  - 示例：
    - `./bin/go-asar extract-file ./app.asar dir1/file1.txt`

- extract
  - 语法：`asar extract <archive> <dest>`
  - 说明：解压整个 `.asar` 到指定目录
  - 示例：
    - `./bin/go-asar extract ./app.asar ./unpacked`

---

## 设计与实现

- 头部格式：
  - 与 `asar` 规范一致：先写入 8 字节的 size-pickle（payload 长度），随后写入 header-pickle（包含 JSON 字符串），再按顺序写入所有“打包文件”的内容。
  - 文件节点中的 `offset` 为内容在文件尾部开始处的偏移（相对于 header 之后的连续数据）。
- 完整性信息：
  - `algorithm: "SHA256"`，`blockSize: 4MB`，`blocks: []string` 逐块哈希，`hash` 为整文件哈希。
- 安全检查：
  - 解包时校验写出路径是否越界（防路径穿越），链接是否指向包外路径。
- 路径相对化：
  - 归档内部记录统一为相对于打包根目录的路径，处理符号链接时优先使用字符串前缀裁切。

---

## CLI 使用

- 打包：

```
./bin/go-asar pack --src <源目录> --dest <输出asar路径>
```

- 解包：

```
./bin/go-asar extract --archive <asar路径> --dest <输出目录>
```

---

## 与 node-asar 的差异

- 已实现核心能力（打包、解包、列出、完整性、头部解析），默认包含隐藏文件。
- `Unpack`/`UnpackDir` 支持常见场景的匹配；如需完全复刻 `minimatch` 的全部语义可在后续扩展。
- “从流打包”（`createPackageFromStreams`）暂未暴露为 Go API，如有需要可继续补充。

---

## 开发与测试

- 构建：`go build ./...`
- 示例验证：可使用 `node-asar/test/input/packthis` 进行打包与解包，并对比 `diff -r`。

```
go build -o ./bin/go-asar ./cmd/asar
./bin/go-asar pack --src ./node-asar/test/input/packthis --dest ./tmp.packthis.asar
./bin/go-asar extract --archive ./tmp.packthis.asar --dest ./tmp.extract
# 可选对比
# diff -r ./tmp.extract ./node-asar/test/input/packthis
```

---

## 许可证

- MIT（与上游一致）。

---

## 致谢

- Electron 官方 `asar` 与其 `node-asar` 参考实现。

---

# English Version

go-asar is a Go implementation of the ASAR pack/unpack library and CLI aligned with Electron’s `asar` (node-asar).

- Pack any app directory to `.asar`
- Extract `.asar` to a destination
- Read and parse ASAR header (Pickle + JSON)
- List all paths in the archive
- Compute file integrity (whole file hash and 4MB block hashes, `SHA256`)

## Install

Library:

```
go get github.com/dcboy/go-asar
```

CLI:

```
go build -o ./bin/go-asar ./cmd/asar
```

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

## Go API

- `CreatePackage(src, dest string) error`
- `CreatePackageWithOptions(src, dest string, options CreateOptions) error`
- `ExtractAll(archivePath, dest string) error`
- `ExtractFile(archivePath, filename string, followLinks bool) ([]byte, error)`
- `ListPackage(archivePath string, isPack bool) ([]string, error)`
- `GetRawHeader(archivePath string) (ArchiveHeader, error)`

## Design

- Header format: size-pickle (payload length) + header-pickle (JSON string), followed by file contents in order
- Integrity: `SHA256` for whole file and 4MB blocks
- Safety: path traversal checks when extracting; symlink target validation
- Relative paths: normalized to archive root; symlinks handled with string prefix trimming then `filepath.Rel` fallback
