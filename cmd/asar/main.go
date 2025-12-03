package main

import (
    "flag"
    "fmt"
    "os"
    "path/filepath"
    "github.com/dcboy/go-asar/asar"
)

func main() {
    if len(os.Args) < 2 {
        fmt.Println("用法: asar pack <src> <dest> | asar extract <archive> <dest>")
        os.Exit(1)
    }
    cmd := os.Args[1]
    switch cmd {
    case "pack":
        fs := flag.NewFlagSet("pack", flag.ExitOnError)
        src := fs.String("src", "", "源目录")
        dest := fs.String("dest", "", "输出 .asar")
        _ = fs.Parse(os.Args[2:])
        if *src == "" || *dest == "" { fmt.Println("请提供 --src 与 --dest"); os.Exit(1) }
        if err := asar.CreatePackage(*src, *dest); err != nil { fmt.Println("打包失败:", err); os.Exit(1) }
        fmt.Println("打包完成:", filepath.Base(*dest))
    case "extract":
        fs := flag.NewFlagSet("extract", flag.ExitOnError)
        archive := fs.String("archive", "", "输入 .asar")
        dest := fs.String("dest", "", "目标目录")
        _ = fs.Parse(os.Args[2:])
        if *archive == "" || *dest == "" { fmt.Println("请提供 --archive 与 --dest"); os.Exit(1) }
        if err := asar.ExtractAll(*archive, *dest); err != nil { fmt.Println("解压失败:", err); os.Exit(1) }
        fmt.Println("解压完成:", *dest)
    default:
        fmt.Println("未知命令:", cmd)
        os.Exit(1)
    }
}

