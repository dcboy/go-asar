package asar

import (
    "encoding/json"
    "errors"
    "io"
    "os"
    "path/filepath"
    "strconv"
)

// FileRecord 文件记录结构（与 Node 版保持一致）
type FileRecord struct {
    FilesystemFileEntry
    Integrity struct {
        Hash      string   `json:"hash"`
        Algorithm string   `json:"algorithm"`
        Blocks    []string `json:"blocks"`
        BlockSize int      `json:"blockSize"`
    } `json:"integrity"`
}

// DirectoryRecord 目录记录结构
type DirectoryRecord struct {
    Files map[string]any `json:"files"`
}

// ArchiveHeader 头部信息
type ArchiveHeader struct {
    Header       FilesystemEntry
    HeaderString string
    HeaderSize   int
}

// ReadArchiveHeaderSync 同步读取 ASAR 头（解析 JSON）
func ReadArchiveHeaderSync(archivePath string) (ArchiveHeader, error) {
    f, err := os.Open(archivePath)
    if err != nil { return ArchiveHeader{}, err }
    defer f.Close()
    sizeBuf := make([]byte, 8)
    if _, err := io.ReadFull(f, sizeBuf); err != nil { return ArchiveHeader{}, errors.New("unable to read header size") }
    sizePickle := NewPickleFromBuffer(sizeBuf)
    size := int(sizePickle.NewIterator().ReadUInt32())
    headerBuf := make([]byte, size)
    if _, err := io.ReadFull(f, headerBuf); err != nil { return ArchiveHeader{}, errors.New("unable to read header") }
    headerPickle := NewPickleFromBuffer(headerBuf)
    headerStr := headerPickle.NewIterator().ReadString()
    hdr, err := decodeHeader([]byte(headerStr))
    if err != nil { return ArchiveHeader{}, err }
    return ArchiveHeader{Header: hdr, HeaderString: headerStr, HeaderSize: size}, nil
}

var filesystemCache = map[string]*Filesystem{}

// ReadFilesystemSync 读取并缓存文件系统头
func ReadFilesystemSync(archivePath string) (*Filesystem, error) {
    if fsys, ok := filesystemCache[archivePath]; ok && fsys != nil { return fsys, nil }
    header, err := ReadArchiveHeaderSync(archivePath)
    if err != nil { return nil, err }
    fsys := NewFilesystem(archivePath)
    fsys.SetHeader(header.Header, header.HeaderSize)
    filesystemCache[archivePath] = fsys
    return fsys, nil
}

// UncacheFilesystem 清理指定缓存
func UncacheFilesystem(archivePath string) bool {
    if _, ok := filesystemCache[archivePath]; ok {
        filesystemCache[archivePath] = nil
        return true
    }
    return false
}

// UncacheAll 清理所有缓存
func UncacheAll() { filesystemCache = map[string]*Filesystem{} }

// ReadFileSync 读取单个文件内容（根据文件条目信息）
func ReadFileSync(fsys *Filesystem, filename string, info *FilesystemFileEntry) ([]byte, error) {
    buffer := make([]byte, info.Size)
    if info.Size <= 0 { return buffer, nil }
    if info.Unpacked {
        return os.ReadFile(filepath.Join(fsys.GetRootPath()+".unpacked", filename))
    }
    fd, err := os.Open(fsys.GetRootPath())
    if err != nil { return nil, err }
    defer fd.Close()
    offset := int64(8 + fsys.GetHeaderSize())
    off, _ := strconv.ParseInt(info.Offset, 10, 64)
    offset += off
    if _, err := fd.ReadAt(buffer, offset); err != nil && err != io.EOF { return nil, err }
    return buffer, nil
}

// createFilesystemWriteStream 创建输出文件并写入 size 与 header pickle
func createFilesystemWriteStream(fsys *Filesystem, dest string) (*os.File, error) {
    headerPickle := NewEmptyPickle()
    // 将 header 序列化为 JSON
    bs, err := json.Marshal(fsys.GetHeader())
    if err != nil { return nil, err }
    headerPickle.WriteString(string(bs))
    headerBuf := headerPickle.ToBuffer()

    sizePickle := NewEmptyPickle()
    sizePickle.WriteUInt32(uint32(len(headerBuf)))
    sizeBuf := sizePickle.ToBuffer()

    out, err := os.Create(dest)
    if err != nil { return nil, err }
    if _, err := out.Write(sizeBuf); err != nil { out.Close(); return nil, err }
    if _, err := out.Write(headerBuf); err != nil { out.Close(); return nil, err }
    return out, nil
}

// createSymlink 在 .unpacked 中创建符号链接
func createSymlink(dest, filepathRel, link string) error {
    base := dest + ".unpacked"
    if err := os.MkdirAll(filepath.Join(base, filepath.Dir(filepathRel)), 0o755); err != nil { return err }
    target := filepath.Join(base, filepathRel)
    if err := os.Symlink(link, target); err != nil { return err }
    return nil
}

// --------- 头解析 ---------

func decodeHeader(bs []byte) (FilesystemEntry, error) {
    var m map[string]any
    if err := json.Unmarshal(bs, &m); err != nil { return nil, err }
    return parseEntry(m)
}

func parseEntry(m map[string]any) (FilesystemEntry, error) {
    // 目录
    if files, ok := m["files"]; ok {
        dir := &FilesystemDirectoryEntry{Files: map[string]FilesystemEntry{}}
        if u, ok := m["unpacked"].(bool); ok { dir.Unpacked = u }
        fm, ok := files.(map[string]any)
        if !ok { return dir, nil }
        for name, child := range fm {
            childMap, ok := child.(map[string]any)
            if !ok { continue }
            ce, err := parseEntry(childMap)
            if err != nil { return nil, err }
            dir.Files[name] = ce
        }
        return dir, nil
    }
    // 链接
    if link, ok := m["link"].(string); ok {
        l := &FilesystemLinkEntry{Link: link}
        if u, ok := m["unpacked"].(bool); ok { l.Unpacked = u }
        return l, nil
    }
    // 文件
    f := &FilesystemFileEntry{}
    if u, ok := m["unpacked"].(bool); ok { f.Unpacked = u }
    if e, ok := m["executable"].(bool); ok { f.Executable = e }
    if off, ok := m["offset"].(string); ok { f.Offset = off }
    if sz, ok := m["size"].(float64); ok { f.Size = int(sz) }
    if integ, ok := m["integrity"].(map[string]any); ok {
        var fi FileIntegrity
        if alg, ok := integ["algorithm"].(string); ok { fi.Algorithm = alg }
        if hash, ok := integ["hash"].(string); ok { fi.Hash = hash }
        if bs, ok := integ["blockSize"].(float64); ok { fi.BlockSize = int(bs) }
        if blks, ok := integ["blocks"].([]any); ok {
            fi.Blocks = make([]string, 0, len(blks))
            for _, b := range blks { if s, ok := b.(string); ok { fi.Blocks = append(fi.Blocks, s) } }
        }
        f.Integrity = fi
    }
    return f, nil
}

// CopyFile 将 srcRoot 下的相对路径 filename 拷贝到 dest.unpacked 下，并保持权限
func CopyFile(destUnpacked string, srcRoot string, filename string) error {
    srcFile := filepath.Join(srcRoot, filename)
    targetFile := filepath.Join(destUnpacked, filename)
    bs, err := os.ReadFile(srcFile)
    if err != nil { return err }
    if err := os.MkdirAll(filepath.Dir(targetFile), 0o755); err != nil { return err }
    fi, err := os.Stat(srcFile)
    if err != nil { return err }
    return os.WriteFile(targetFile, bs, fi.Mode())
}
