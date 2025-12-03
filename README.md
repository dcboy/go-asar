# go-asar

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
