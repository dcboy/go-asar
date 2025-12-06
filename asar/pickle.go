package asar

import (
	"encoding/binary"
)

const (
	sizeInt32   = 4
	sizeUint32  = 4
	payloadUnit = 64
)

// alignInt 将整数按 alignment 对齐到上一个倍数
func alignInt(i, alignment int) int {
	return i + ((alignment - (i % alignment)) % alignment)
}

// Pickle 表示 Chromium 的二进制打包结构，支持基本类型的写入与读取
// Pickle 的前 4 字节为 payload 大小（uint32 LE），随后为按 4 字节对齐的 payload
type Pickle struct {
	header              []byte
	headerSize          int
	capacityAfterHeader int
	writeOffset         int
}

// NewEmptyPickle 创建一个空的 Pickle 用于写入
func NewEmptyPickle() *Pickle {
	p := &Pickle{
		header:              make([]byte, 4),
		headerSize:          4,
		capacityAfterHeader: 0,
		writeOffset:         0,
	}
	p.resize(payloadUnit)
	p.setPayloadSize(0)
	return p
}

// NewPickleFromBuffer 从完整的 Pickle 缓冲读取结构（含 header 和 payload）
func NewPickleFromBuffer(buf []byte) *Pickle {
	p := &Pickle{
		header: buf,
	}
	payload := int(p.getPayloadSize())
	p.headerSize = len(buf) - payload
	if p.headerSize < 0 || p.headerSize != alignInt(p.headerSize, sizeUint32) {
		p.headerSize = 0
		p.header = []byte{}
	}
	p.capacityAfterHeader = int(9_007_199_254_740_992) // CAPACITY_READ_ONLY
	p.writeOffset = 0
	return p
}

// ToBuffer 导出当前 Pickle 的有效字节切片
func (p *Pickle) ToBuffer() []byte {
	return p.header[:p.headerSize+p.getPayloadSize()]
}

// Iterator 提供对 Pickle 的顺序读取能力
type Iterator struct {
	payload       []byte
	payloadOffset int
	readIndex     int
	endIndex      int
}

// NewIterator 创建读取迭代器
func (p *Pickle) NewIterator() *Iterator {
	it := &Iterator{
		payload:       p.header,
		payloadOffset: p.headerSize,
		readIndex:     0,
		endIndex:      p.getPayloadSize(),
	}
	return it
}

// ReadUInt32 读取一个 uint32（LE）
func (it *Iterator) ReadUInt32() uint32 {
	var v uint32
	it.readBytes(sizeUint32, func(b []byte) { v = binary.LittleEndian.Uint32(b) })
	return v
}

// ReadInt32 读取一个 int32（LE）
func (it *Iterator) ReadInt32() int32 {
	var v int32
	it.readBytes(sizeInt32, func(b []byte) { v = int32(binary.LittleEndian.Uint32(b)) })
	return v
}

// ReadString 读取一个以长度前缀的字符串（utf8）
func (it *Iterator) ReadString() string {
	l := int(it.ReadInt32())
	var s []byte
	it.readBytes(l, func(b []byte) { s = append([]byte(nil), b...) })
	return string(s)
}

func (it *Iterator) readBytes(length int, fn func([]byte)) {
	if length > it.endIndex-it.readIndex {
		it.readIndex = it.endIndex
		panic("pickle: insufficient payload to read")
	}
	off := it.payloadOffset + it.readIndex
	fn(it.payload[off : off+length])
	it.advance(length)
}

func (it *Iterator) advance(size int) {
	aligned := alignInt(size, sizeUint32)
	if it.endIndex-it.readIndex < aligned {
		it.readIndex = it.endIndex
	} else {
		it.readIndex += aligned
	}
}

// WriteUInt32 写入一个 uint32（LE）
func (p *Pickle) WriteUInt32(v uint32) bool {
	return p.writeBytes(func(buf []byte) { binary.LittleEndian.PutUint32(buf, v) }, sizeUint32)
}

// WriteInt32 写入一个 int32（LE）
func (p *Pickle) WriteInt32(v int32) bool {
	return p.writeBytes(func(buf []byte) { binary.LittleEndian.PutUint32(buf, uint32(v)) }, sizeInt32)
}

// WriteString 写入一个以长度前缀的字符串（utf8）
func (p *Pickle) WriteString(s string) bool {
	if !p.WriteInt32(int32(len(s))) {
		return false
	}
	// 原样写入字符串并进行 4 字节对齐填充
	return p.writeRawBytes([]byte(s))
}

func (p *Pickle) writeRawBytes(data []byte) bool {
	length := len(data)
	dataLength := alignInt(length, sizeUint32)
	newSize := p.writeOffset + dataLength
	if newSize > p.capacityAfterHeader {
		p.resize(max(p.capacityAfterHeader*2, newSize))
	}
	copy(p.header[p.headerSize+p.writeOffset:], data)
	// 对齐填充 0
	end := p.headerSize + p.writeOffset + length
	for i := end; i < p.headerSize+p.writeOffset+dataLength; i++ {
		p.header[i] = 0
	}
	p.setPayloadSize(newSize)
	p.writeOffset = newSize
	return true
}

func (p *Pickle) writeBytes(writer func([]byte), length int) bool {
	dataLength := alignInt(length, sizeUint32)
	newSize := p.writeOffset + dataLength
	if newSize > p.capacityAfterHeader {
		p.resize(max(p.capacityAfterHeader*2, newSize))
	}
	writer(p.header[p.headerSize+p.writeOffset : p.headerSize+p.writeOffset+length])
	// 对齐填充 0
	end := p.headerSize + p.writeOffset + length
	for i := end; i < p.headerSize+p.writeOffset+dataLength; i++ {
		p.header[i] = 0
	}
	p.setPayloadSize(newSize)
	p.writeOffset = newSize
	return true
}

func (p *Pickle) setPayloadSize(sz int) int {
	binary.LittleEndian.PutUint32(p.header[:4], uint32(sz))
	return sz
}

func (p *Pickle) getPayloadSize() int {
	return int(binary.LittleEndian.Uint32(p.header[:4]))
}

func (p *Pickle) resize(newCapacity int) {
	newCapacity = alignInt(newCapacity, payloadUnit)
	buf := make([]byte, p.headerSize+newCapacity)
	copy(buf, p.header)
	p.header = buf
	p.capacityAfterHeader = newCapacity
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
