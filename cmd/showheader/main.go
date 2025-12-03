package main

import (
    "fmt"
    "os"
    "github.com/dcboy/go-asar/asar"
)

func main() {
    if len(os.Args) < 2 { fmt.Println("用法: showheader <archive>"); os.Exit(1) }
    hdr, err := asar.GetRawHeader(os.Args[1])
    if err != nil { fmt.Println("读取失败:", err); os.Exit(1) }
    fmt.Println("headerSize:", hdr.HeaderSize)
    fmt.Println("headerString:")
    fmt.Println(hdr.HeaderString)
}

