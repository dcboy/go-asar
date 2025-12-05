package asar

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// CreateOptions 打包选项
type CreateOptions struct {
	Dot       bool
	Ordering  string
	Pattern   string
	Transform func(filePath string) io.ReadCloser
	Unpack    string
	UnpackDir string
}

// isUnpackedDir 判断目录是否匹配 unpackDir 规则（支持前缀或简易 glob）
func isUnpackedDir(dirPath, pattern string, unpackDirs *[]string) bool {
	if strings.HasPrefix(dirPath, pattern) || matchGlob(dirPath, pattern) {
		if !contains(*unpackDirs, dirPath) {
			*unpackDirs = append(*unpackDirs, dirPath)
		}
		return true
	}
	for _, up := range *unpackDirs {
		if strings.HasPrefix(dirPath, up) && !strings.HasPrefix(relPath(up, dirPath), "..") {
			return true
		}
	}
	return false
}

// CreatePackage 以默认选项打包目录
func CreatePackage(src, dest string) error {
	return CreatePackageWithOptions(src, dest, CreateOptions{})
}

// CreatePackageWithOptions 根据选项打包目录
func CreatePackageWithOptions(src, dest string, options CreateOptions) error {
	if options.Pattern == "" {
		options.Pattern = "/**/*"
	}
	absSrc, _ := filepath.Abs(src)
	includeDot := options.Dot
	files, meta, err := Crawl(filepath.Join(absSrc), includeDot)
	if err != nil {
		return err
	}
	return CreatePackageFromFiles(absSrc, dest, files, meta, options)
}

// CreatePackageFromFiles 从文件列表创建 ASAR
func CreatePackageFromFiles(src, dest string, filenames []string, metadata map[string]*CrawledFileType, options CreateOptions) error {
	src, _ = filepath.Abs(src)
	dest, _ = filepath.Abs(dest)
	for i, f := range filenames {
		filenames[i] = filepath.Clean(f)
	}

	root := &FilesystemDirectoryEntry{Files: map[string]FilesystemEntry{}}
	files := make([]struct {
		filename string
		unpack   bool
	}, 0)
	links := make([]struct {
		filename string
		unpack   bool
	}, 0)
	unpackDirs := make([]string, 0)
	var offset int64 = 0

	filenamesSorted := filenames
	if options.Ordering != "" {
		bs, err := os.ReadFile(options.Ordering)
		if err == nil {
			lines := strings.Split(string(bs), "\n")
			ordering := make([]string, 0)
			for _, line := range lines {
				if i := strings.Index(line, ":"); i >= 0 {
					line = strings.TrimSpace(line[i+1:])
				} else {
					line = strings.TrimSpace(line)
				}
				line = strings.TrimPrefix(line, "/")
				comps := strings.Split(line, string(os.PathSeparator))
				cur := src
				for _, c := range comps {
					cur = filepath.Join(cur, c)
					ordering = append(ordering, cur)
				}
			}
			filenamesSorted = make([]string, 0, len(filenames))
			missing := 0
			total := len(filenames)
			for _, f := range ordering {
				if !contains(filenamesSorted, f) && contains(filenames, f) {
					filenamesSorted = append(filenamesSorted, f)
				}
			}
			for _, f := range filenames {
				if !contains(filenamesSorted, f) {
					filenamesSorted = append(filenamesSorted, f)
					missing++
				}
			}
			_ = total
			_ = missing
		}
	}

	shouldUnpackPath := func(relativePath, unpack, unpackDir string) bool {
		su := false
		if unpack != "" {
			su = matchBase(relativePath, unpack)
		}
		if !su && unpackDir != "" {
			su = isUnpackedDir(relativePath, unpackDir, &unpackDirs)
		}
		return su
	}

	handleFile := func(filename string) error {
		m, ok := metadata[filename]
		if !ok {
			typ, err := DetermineFileType(filename)
			if err != nil {
				return err
			}
			metadata[filename] = typ
			m = typ
		}
		relAll := relPath(src, filename)
		if !options.Dot {
			if strings.HasPrefix(filepath.Base(relAll), ".") {
				if m.Type == "directory" {
					return nil
				}
				return nil
			}
		}
		switch m.Type {
		case "directory":
			su := shouldUnpackPath(relAll, "", options.UnpackDir)
			ensureDir(root, relAll, su)
		case "file":
			su := shouldUnpackPath(relPath(src, filepath.Dir(filename)), options.Unpack, options.UnpackDir)
			files = append(files, struct {
				filename string
				unpack   bool
			}{filename, su})
			// 创建文件节点并填充元数据
			rel := relAll
			dir := ensureDir(root, relPath(src, filepath.Dir(filename)), false)
			name := filepath.Base(rel)
			fe := &FilesystemFileEntry{Unpacked: su, Size: int(m.Stat.Size())}
			f, err := os.Open(filename)
			if err != nil {
				return err
			}
			integ, err := GetFileIntegrity(f)
			f.Close()
			if err != nil {
				return err
			}
			fe.Integrity = integ
			if !isWindows() && (m.Stat.Mode()&0o100) != 0 {
				fe.Executable = true
			}
			if !su {
				fe.Offset = strconv.FormatInt(offset, 10)
				offset += int64(fe.Size)
			}
			dir[name] = fe
		case "link":
			su := shouldUnpackPath(relPath(src, filename), options.Unpack, options.UnpackDir)
			links = append(links, struct {
				filename string
				unpack   bool
			}{filename, su})
			// 插入链接
			rel := relPath(src, filename)
			dir := ensureDir(root, relPath(src, filepath.Dir(filename)), false)
			linkTarget := relPath(mustRealpath(src), filepath.Join(mustRealpath(filepath.Dir(filename)), mustReadlink(filename)))
			le := &FilesystemLinkEntry{Link: linkTarget, EntryMetadata: EntryMetadata{Unpacked: su}}
			dir[filepath.Base(rel)] = le
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	// 先构建头与偏移
	for _, name := range filenamesSorted {
		if err := handleFile(name); err != nil {
			return err
		}
	}
	// 写入 header
	tmpFS := &Filesystem{}
	tmpFS.SetHeader(root, 0)
	out, err := createFilesystemWriteStream(tmpFS, dest)
	if err != nil {
		return err
	}
	defer out.Close()

	for _, f := range files {
		if f.unpack {
			filename := relPath(src, f.filename)
			if err := CopyFile(dest+".unpacked", src, filename); err != nil {
				return err
			}
		} else {
			in, err := os.Open(f.filename)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, in); err != nil {
				in.Close()
				return err
			}
			in.Close()
		}
	}
	for _, l := range links {
		if l.unpack {
			filename := relPath(src, l.filename)
			if err := createSymlink(dest, filename, mustReadlink(l.filename)); err != nil {
				return err
			}
		}
	}
	return nil
}

// StatFile 获取单个条目信息
func StatFile(archivePath, filename string, followLinks bool) (FilesystemEntry, error) {
	fsys, err := ReadFilesystemSync(archivePath)
	if err != nil {
		return nil, err
	}
	return fsys.GetFile(filename, followLinks)
}

// GetRawHeader 返回原始头信息
func GetRawHeader(archivePath string) (ArchiveHeader, error) {
	return ReadArchiveHeaderSync(archivePath)
}

// ListPackage 列出包内容
func ListPackage(archivePath string, isPack bool) ([]string, error) {
	fsys, err := ReadFilesystemSync(archivePath)
	if err != nil {
		return nil, err
	}
	return fsys.ListFiles(isPack), nil
}

// ExtractFile 提取单个文件内容
func ExtractFile(archivePath, filename string, followLinks bool) ([]byte, error) {
	fsys, err := ReadFilesystemSync(archivePath)
	if err != nil {
		return nil, err
	}
	fi, err := fsys.GetFile(filename, followLinks)
	if err != nil {
		return nil, err
	}
	f, ok := fi.(*FilesystemFileEntry)
	if !ok {
		return nil, errors.New("not a file: " + filename)
	}
	return ReadFileSync(fsys, filename, f)
}

// ExtractAll 提取全部文件到目标目录
func ExtractAll(archivePath, dest string) error {
	fsys, err := ReadFilesystemSync(archivePath)
	if err != nil {
		return err
	}
	filenames := fsys.ListFiles(false)
	followLinks := os.PathSeparator == '\\' // Windows 提取为普通文件
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	for _, full := range filenames {
		filename := strings.TrimPrefix(full, "/")
		destFilename := filepath.Join(dest, filename)
		if isOutOf(dest, destFilename) {
			return errors.New(full + ": file \"" + destFilename + "\" writes out of the package")
		}
		fileEntry, _ := fsys.GetFile(filename, followLinks)
		if dir, ok := fileEntry.(*FilesystemDirectoryEntry); ok {
			_ = dir
			if err := os.MkdirAll(destFilename, 0o755); err != nil {
				return err
			}
		} else if lnk, ok := fileEntry.(*FilesystemLinkEntry); ok {
			linkSrc := filepath.Dir(filepath.Join(dest, lnk.Link))
			linkDest := filepath.Dir(destFilename)
			rel := relPath(linkDest, linkSrc)
			_ = os.Remove(destFilename)
			linkTo := filepath.Join(rel, filepath.Base(lnk.Link))
			if isOutOf(dest, linkSrc) {
				return errors.New(full + ": file \"" + lnk.Link + "\" links out of the package to \"" + linkSrc + "\"")
			}
			if err := os.Symlink(linkTo, destFilename); err != nil {
				return err
			}
		} else if f, ok := fileEntry.(*FilesystemFileEntry); ok {
			content, err := ReadFileSync(fsys, filename, f)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(destFilename), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(destFilename, content, 0o644); err != nil {
				return err
			}
			if f.Executable {
				_ = os.Chmod(destFilename, 0o755)
			}
		}
	}
	return nil
}

// ------------- 辅助函数 -------------

func matchBase(pathRel, pattern string) bool {
	// 简化实现：仅匹配 basename
	return matchGlob(filepath.Base(pathRel), pattern)
}

func matchGlob(name, pattern string) bool {
	ok, _ := filepath.Match(pattern, name)
	return ok
}

func contains[T comparable](arr []T, v T) bool {
	for _, x := range arr {
		if x == v {
			return true
		}
	}
	return false
}

func relPath(base, target string) string {
	base = filepath.Clean(base)
	target = filepath.Clean(target)
	// 优先使用字符串前缀去除，避免符号链接差异导致 '.'
	prefix := base + string(os.PathSeparator)
	if strings.HasPrefix(target, prefix) {
		return filepath.ToSlash(strings.TrimPrefix(target, prefix))
	}
	r, _ := filepath.Rel(base, target)
	return filepath.ToSlash(r)
}

func isOutOf(base, p string) bool {
	r, _ := filepath.Rel(base, p)
	return strings.HasPrefix(filepath.ToSlash(r), "..")
}

func mustStat(p string) os.FileInfo { fi, _ := os.Lstat(p); return fi }

func mustRealpath(p string) string {
	a, _ := filepath.EvalSymlinks(p)
	if a == "" {
		a, _ = filepath.Abs(p)
	}
	return a
}

func mustReadlink(p string) string { t, _ := os.Readlink(p); return t }

// ensureDir 根据相对路径创建并返回对应目录的 Files 映射
func ensureDir(root *FilesystemDirectoryEntry, rel string, markUnpack bool) map[string]FilesystemEntry {
	cur := root
	if rel == "." || rel == "" {
		return cur.Files
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for _, part := range parts {
		if part == "" {
			continue
		}
		next, ok := cur.Files[part]
		if !ok {
			d := &FilesystemDirectoryEntry{Files: map[string]FilesystemEntry{}}
			cur.Files[part] = d
			cur = d
		} else if dd, isDir := next.(*FilesystemDirectoryEntry); isDir {
			cur = dd
		} else {
			// 覆盖为目录
			d := &FilesystemDirectoryEntry{Files: map[string]FilesystemEntry{}}
			cur.Files[part] = d
			cur = d
		}
	}
	if markUnpack {
		cur.Unpacked = true
	}
	return cur.Files
}

func isWindows() bool { return os.PathSeparator == '\\' }
