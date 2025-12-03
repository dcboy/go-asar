package asar

import (
    "io"
    "os"
    "path/filepath"
)

// CrawledFileType 表示爬取到的条目信息
type CrawledFileType struct {
    Type string     // file | directory | link
    Stat os.FileInfo
}

// DetermineFileType 判断文件类型
func DetermineFileType(filename string) (*CrawledFileType, error) {
    fi, err := os.Lstat(filename)
    if err != nil { return nil, err }
    if fi.Mode()&os.ModeSymlink != 0 { return &CrawledFileType{Type: "link", Stat: fi}, nil }
    if fi.IsDir() { return &CrawledFileType{Type: "directory", Stat: fi}, nil }
    return &CrawledFileType{Type: "file", Stat: fi}, nil
}

// Crawl 递归遍历目录，返回路径列表与元数据
func Crawl(root string, includeDot bool) ([]string, map[string]*CrawledFileType, error) {
    meta := map[string]*CrawledFileType{}
    files := make([]string, 0)
    err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
        if err != nil { return err }
        // 过滤隐藏文件（可选）
        if !includeDot {
            base := filepath.Base(path)
            if len(base) > 0 && base[0] == '.' && path != root { return nil }
        }
        if path == root { return nil }
        typ, err := DetermineFileType(path)
        if err != nil { return err }
        meta[path] = typ
        files = append(files, path)
        return nil
    })
    if err != nil { return nil, nil, err }
    return files, meta, nil
}

// StreamGenerator 为生成读取流的函数类型
type StreamGenerator func() (io.ReadCloser, error)
