package asar

import (
    "errors"
    "io"
    "os"
    "path/filepath"
    "strconv"
)

// EntryMetadata 表示通用条目元数据
type EntryMetadata struct {
    Unpacked bool `json:"unpacked,omitempty"`
}

// FilesystemDirectoryEntry 目录条目
type FilesystemDirectoryEntry struct {
    Files map[string]FilesystemEntry `json:"files"`
    EntryMetadata
}

// FilesystemFileEntry 文件条目
type FilesystemFileEntry struct {
    Unpacked   bool          `json:"unpacked"`
    Executable bool          `json:"executable"`
    Offset     string        `json:"offset"`
    Size       int           `json:"size"`
    Integrity  FileIntegrity `json:"integrity"`
    EntryMetadata
}

// FilesystemLinkEntry 符号链接条目
type FilesystemLinkEntry struct {
    Link string `json:"link"`
    EntryMetadata
}

// FilesystemEntry 泛型条目
type FilesystemEntry interface{}

// Filesystem 表示 ASAR 文件系统头与构建状态
type Filesystem struct {
    src        string
    header     FilesystemEntry
    headerSize int
    offset     int64
}

// NewFilesystem 创建新的文件系统对象
func NewFilesystem(src string) *Filesystem {
    abs, _ := filepath.Abs(src)
    return &Filesystem{
        src:        abs,
        header:     &FilesystemDirectoryEntry{Files: map[string]FilesystemEntry{}},
        headerSize: 0,
        offset:     0,
    }
}

// GetRootPath 返回根路径
func (fsys *Filesystem) GetRootPath() string { return fsys.src }

// GetHeader 返回头对象
func (fsys *Filesystem) GetHeader() FilesystemEntry { return fsys.header }

// GetHeaderSize 返回头大小
func (fsys *Filesystem) GetHeaderSize() int { return fsys.headerSize }

// SetHeader 设置头与头大小
func (fsys *Filesystem) SetHeader(h FilesystemEntry, size int) { fsys.header = h; fsys.headerSize = size }

// searchNodeFromDirectory 按目录路径查找或创建节点
func (fsys *Filesystem) searchNodeFromDirectory(p string) FilesystemEntry {
    json := fsys.header
    parts := filepath.SplitList(filepath.ToSlash(p))
    // filepath.SplitList 不适用于一般路径分割，这里手动按 '/'
    parts = splitPath(p)
    for _, dir := range parts {
        if dir == "." || dir == "" { continue }
        dirEntry, ok := json.(*FilesystemDirectoryEntry)
        if !ok { panic("unexpected directory state: " + p) }
        if dirEntry.Files == nil { dirEntry.Files = map[string]FilesystemEntry{} }
        child, exists := dirEntry.Files[dir]
        if !exists {
            child = &FilesystemDirectoryEntry{Files: map[string]FilesystemEntry{}}
            dirEntry.Files[dir] = child
        }
        json = child
    }
    return json
}

// searchNodeFromPath 按完整路径查找或创建节点
func (fsys *Filesystem) searchNodeFromPath(p string) FilesystemEntry {
    rel, _ := filepath.Rel(fsys.src, p)
    if rel == "." || rel == "" { return fsys.header }
    name := filepath.Base(rel)
    dir := filepath.Dir(rel)
    node := fsys.searchNodeFromDirectory(dir).(*FilesystemDirectoryEntry)
    if node.Files == nil { node.Files = map[string]FilesystemEntry{} }
    child, ok := node.Files[name]
    if !ok {
        child = &FilesystemFileEntry{}
        node.Files[name] = child
    }
    return child
}

// InsertDirectory 插入目录节点，支持 unpack 标记
func (fsys *Filesystem) InsertDirectory(p string, shouldUnpack bool) map[string]FilesystemEntry {
    rel, _ := filepath.Rel(fsys.src, p)
    parent := fsys.searchNodeFromDirectory(filepath.Dir(rel)).(*FilesystemDirectoryEntry)
    name := filepath.Base(rel)
    child, ok := parent.Files[name]
    var node *FilesystemDirectoryEntry
    if ok {
        if d, isDir := child.(*FilesystemDirectoryEntry); isDir {
            node = d
        } else {
            // 覆盖为目录
            node = &FilesystemDirectoryEntry{Files: map[string]FilesystemEntry{}}
            parent.Files[name] = node
        }
    } else {
        node = &FilesystemDirectoryEntry{Files: map[string]FilesystemEntry{}}
        parent.Files[name] = node
    }
    if shouldUnpack { node.Unpacked = true }
    if node.Files == nil { node.Files = map[string]FilesystemEntry{} }
    return node.Files
}

// InsertFile 插入文件节点并更新偏移、完整性等信息
func (fsys *Filesystem) InsertFile(p string, streamGenerator func() (ioReadSeeker, error), shouldUnpack bool, fileStat os.FileInfo) error {
    dirNode := fsys.searchNodeFromDirectory(filepath.Dir(p)).(*FilesystemDirectoryEntry)
    // 确保文件节点类型为文件
    rel, _ := filepath.Rel(fsys.src, p)
    parent := fsys.searchNodeFromDirectory(filepath.Dir(rel)).(*FilesystemDirectoryEntry)
    name := filepath.Base(rel)
    // 直接覆盖为文件条目，确保类型正确
    node := &FilesystemFileEntry{}
    parent.Files[name] = node
    if shouldUnpack || dirNode.Unpacked {
        node.Size = int(fileStat.Size())
        node.Unpacked = true
        r, err := streamGenerator()
        if err != nil { return err }
        integ, err := GetFileIntegrity(r)
        if err != nil { return err }
        node.Integrity = integ
        return nil
    }
    size := int(fileStat.Size())
    if uint64(size) > uint64(^uint32(0)) {
        return errors.New(p + ": file size can not be larger than 4.2GB")
    }
    node.Size = size
    node.Offset = int64ToString(fsys.offset)
    r, err := streamGenerator()
    if err != nil { return err }
    integ, err := GetFileIntegrity(r)
    if err != nil { return err }
    node.Integrity = integ
    // 可执行位（非 Windows）
    if isExecutable(fileStat) { node.Executable = true }
    fsys.offset += int64(size)
    return nil
}

// InsertLink 插入符号链接节点，校验链接不越界
func (fsys *Filesystem) InsertLink(p string, shouldUnpack bool, parentPath string, symlink string, src string) (string, error) {
    link := fsys.resolveLink(src, parentPath, symlink)
    if hasParentOutOf(src, link) {
        return "", errors.New(p + ": file \"" + link + "\" links out of the package")
    }
    node := fsys.searchNodeFromPath(p).(*FilesystemLinkEntry)
    dirNode := fsys.searchNodeFromDirectory(filepath.Dir(p)).(*FilesystemDirectoryEntry)
    if shouldUnpack || dirNode.Unpacked { node.Unpacked = true }
    node.Link = link
    return link, nil
}

// ListFiles 列出所有路径，可选是否包含 pack/unpack 前缀
func (fsys *Filesystem) ListFiles(isPack bool) []string {
    files := make([]string, 0)
    var fill func(base string, meta FilesystemEntry)
    fill = func(base string, meta FilesystemEntry) {
        dir, ok := meta.(*FilesystemDirectoryEntry)
        if !ok { return }
        for childPath, childMeta := range dir.Files {
            full := filepath.ToSlash(filepath.Join(base, childPath))
            if isPack {
                state := "pack  "
                if hasUnpacked(childMeta) { state = "unpack" }
                files = append(files, state+" : "+full)
            } else {
                files = append(files, full)
            }
            fill(full, childMeta)
        }
    }
    fill("/", fsys.header)
    return files
}

// GetNode 获取任意路径的条目（可解析符号链接）
func (fsys *Filesystem) GetNode(p string, followLinks bool) FilesystemEntry {
    node := fsys.searchNodeFromDirectory(filepath.Dir(p))
    name := filepath.Base(p)
    if lnk, ok := node.(*FilesystemLinkEntry); ok && followLinks {
        return fsys.GetNode(filepath.Join(lnk.Link, name), followLinks)
    }
    if name != "" {
        dir := node.(*FilesystemDirectoryEntry)
        return dir.Files[name]
    }
    return node
}

// GetFile 获取文件条目（可解析符号链接）
func (fsys *Filesystem) GetFile(p string, followLinks bool) (FilesystemEntry, error) {
    info := fsys.GetNode(p, followLinks)
    if info == nil { return nil, errors.New("\"" + p + "\" was not found in this archive") }
    if lnk, ok := info.(*FilesystemLinkEntry); ok && followLinks {
        return fsys.GetFile(lnk.Link, followLinks)
    }
    return info, nil
}

// resolveLink 计算符号链接的相对路径
func (fsys *Filesystem) resolveLink(src, parentPath, symlink string) string {
    target := filepath.Join(parentPath, symlink)
    link, _ := filepath.Rel(src, target)
    return filepath.ToSlash(link)
}

// 辅助函数
func splitPath(p string) []string {
    p = filepath.ToSlash(p)
    if p == "." || p == "" { return nil }
    parts := make([]string, 0)
    for _, s := range bytesSplit(p, '/') {
        if s != "" { parts = append(parts, s) }
    }
    return parts
}

func bytesSplit(s string, sep byte) []string {
    b := []byte(s)
    out := make([]string, 0)
    start := 0
    for i := 0; i < len(b); i++ {
        if b[i] == sep {
            out = append(out, string(b[start:i]))
            start = i + 1
        }
    }
    out = append(out, string(b[start:]))
    return out
}

func int64ToString(v int64) string { return fmtInt64(v) }

func fmtInt64(v int64) string { return strconv.FormatInt(v, 10) }

func isExecutable(fi os.FileInfo) bool {
    return (fi.Mode() & 0o100) != 0 && os.PathSeparator == '/'
}

func hasUnpacked(e FilesystemEntry) bool {
    switch t := e.(type) {
    case *FilesystemDirectoryEntry:
        return t.Unpacked
    case *FilesystemFileEntry:
        return t.Unpacked
    case *FilesystemLinkEntry:
        return t.Unpacked
    default:
        return false
    }
}

func hasParentOutOf(base, rel string) bool {
    // 如果 rel 以 ".." 开头则认为越界
    return len(rel) >= 2 && rel[:2] == ".."
}

// ioReadSeeker 为通用读取接口
type ioReadSeeker interface {
    io.Reader
}
