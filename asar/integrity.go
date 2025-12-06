package asar

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
)

const (
	// ALGORITHM 哈希算法标识
	ALGORITHM = "SHA256"
	// BLOCK_SIZE 分块大小（4MB）
	BLOCK_SIZE = 4 * 1024 * 1024
)

// FileIntegrity 表示文件完整性信息
type FileIntegrity struct {
	Algorithm string   `json:"algorithm"`
	Hash      string   `json:"hash"`
	BlockSize int      `json:"blockSize"`
	Blocks    []string `json:"blocks"`
}

// GetFileIntegrity 计算输入流的完整性信息，包含整文件哈希与分块哈希
func GetFileIntegrity(r io.Reader) (FileIntegrity, error) {
	h := sha256.New()
	blocks := make([]string, 0)
	buf := make([]byte, BLOCK_SIZE)
	for {
		n, err := io.ReadFull(r, buf)
		if err == io.ErrUnexpectedEOF {
			if n > 0 {
				hb := sha256.Sum256(buf[:n])
				blocks = append(blocks, hex.EncodeToString(hb[:]))
				h.Write(buf[:n])
			}
			break
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return FileIntegrity{}, err
		}
		hb := sha256.Sum256(buf[:n])
		blocks = append(blocks, hex.EncodeToString(hb[:]))
		h.Write(buf[:n])
	}
	sum := h.Sum(nil)
	return FileIntegrity{
		Algorithm: ALGORITHM,
		Hash:      hex.EncodeToString(sum),
		BlockSize: BLOCK_SIZE,
		Blocks:    blocks,
	}, nil
}
