package main

import (
    "flag"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "github.com/dcboy/go-asar/asar"
)

// main 解析命令并对齐 node-asar 的子命令与参数
func main() {
    if len(os.Args) < 2 {
        printHelp()
        os.Exit(1)
    }
    cmd := os.Args[1]
    switch cmd {
    case "pack", "p":
        // 手工解析，支持选项与位置参数交错
        dir, output, opts := parsePackArgs(os.Args[2:])
        if dir == "" || output == "" { fmt.Println("用法: asar pack <dir> <output> [options]"); os.Exit(1) }
        if err := asar.CreatePackageWithOptions(dir, output, opts); err != nil {
            fmt.Println("打包失败:", err)
            os.Exit(1)
        }
        fmt.Println("打包完成:", filepath.Base(output))
    case "list", "l":
        archive, isPack := parseListArgs(os.Args[2:])
        if archive == "" { fmt.Println("用法: asar list <archive> [-i]"); os.Exit(1) }
        files, err := asar.ListPackage(archive, isPack)
        if err != nil { fmt.Println("读取失败:", err); os.Exit(1) }
        for _, f := range files { fmt.Println(f) }
    case "extract-file", "ef":
        // extract-file <archive> <filename>
        fs := flag.NewFlagSet("extract-file", flag.ExitOnError)
        _ = fs.Parse(os.Args[2:])
        args := fs.Args()
        if len(args) < 2 { fmt.Println("用法: asar extract-file <archive> <filename>"); os.Exit(1) }
        archive := args[0]
        filename := args[1]
        data, err := asar.ExtractFile(archive, filename, true)
        if err != nil { fmt.Println("提取失败:", err); os.Exit(1) }
        if err := os.WriteFile(filepath.Base(filename), data, 0o644); err != nil { fmt.Println("写入失败:", err); os.Exit(1) }
    case "extract", "e":
        // extract <archive> <dest>
        fs := flag.NewFlagSet("extract", flag.ExitOnError)
        _ = fs.Parse(os.Args[2:])
        args := fs.Args()
        if len(args) < 2 { fmt.Println("用法: asar extract <archive> <dest>"); os.Exit(1) }
        archive := args[0]
        dest := args[1]
        if err := asar.ExtractAll(archive, dest); err != nil { fmt.Println("解压失败:", err); os.Exit(1) }
        fmt.Println("解压完成:", dest)
    default:
        printHelp()
        os.Exit(1)
    }
}

// printHelp 打印简单帮助
func printHelp() {
    fmt.Println("用法:")
    fmt.Println("  asar pack <dir> <output> [--ordering --unpack --unpack-dir --exclude-hidden]")
    fmt.Println("  asar list <archive> [-i | --is-pack]")
    fmt.Println("  asar extract-file <archive> <filename>")
    fmt.Println("  asar extract <archive> <dest>")
}

// parsePackArgs 解析 pack 子命令参数，支持交错
func parsePackArgs(argv []string) (string, string, asar.CreateOptions) {
    var dir, output string
    opts := asar.CreateOptions{Dot: true}
    for i := 0; i < len(argv); i++ {
        a := argv[i]
        if a == "--ordering" && i+1 < len(argv) { opts.Ordering = argv[i+1]; i++ } else
        if a == "--unpack" && i+1 < len(argv) { opts.Unpack = argv[i+1]; i++ } else
        if a == "--unpack-dir" && i+1 < len(argv) { opts.UnpackDir = argv[i+1]; i++ } else
        if a == "--exclude-hidden" { opts.Dot = false } else if strings.HasPrefix(a, "-") {
            // 忽略未知选项
        } else if dir == "" { dir = a } else if output == "" { output = a }
    }
    return dir, output, opts
}

// parseListArgs 解析 list 子命令参数
func parseListArgs(argv []string) (string, bool) {
    var archive string
    var isPack bool
    for i := 0; i < len(argv); i++ {
        a := argv[i]
        if a == "-i" || a == "--is-pack" { isPack = true } else if strings.HasPrefix(a, "-") {
        } else if archive == "" { archive = a }
    }
    return archive, isPack
}
