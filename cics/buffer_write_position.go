package cics

import "unsafe"

type BufferWritePosition struct {
	Buf []byte
}

func (b *BufferWritePosition) WriteStringPosition(s string, i int) {
	lastByte := i + len(s)
	if b.Cap() < lastByte {
		b.growLen(lastByte - b.Cap())
	}
	copy(b.Buf[i:lastByte], []byte(s))

}

func (b *BufferWritePosition) Cap() int { return cap(b.Buf) }

func (b *BufferWritePosition) Len() int { return len(b.Buf) }

func (b *BufferWritePosition) grow(n int) {
	buf := make([]byte, len(b.Buf), 2*cap(b.Buf)+n)
	copy(buf, b.Buf)
	b.Buf = buf
}
func (b *BufferWritePosition) growLen(n int) {
	buf := make([]byte, 2*len(b.Buf)+n)
	copy(buf, b.Buf)
	b.Buf = buf

}
func (b *BufferWritePosition) WriteString(s string) (int, error) {
	b.Buf = append(b.Buf, s...)
	return len(s), nil
}

func (b *BufferWritePosition) String() string {
	return unsafe.String(unsafe.SliceData(b.Buf), len(b.Buf))

}
