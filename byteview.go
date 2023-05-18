package weekendns

import (
	"fmt"
)

// ByteView is a simplistic wrapper around a slice of bytes that maintains an
// internal index into its data and exposes a vaguely io.Reader-esque API.
//
// The idea is that an incoming DNS packet will be read entirely into memory,
// and this will be an efficient way to index into it while maintaining a
// pointer to the most recently read byte.
type ByteView struct {
	data   []byte
	offset uint16
}

func NewByteView(data []byte) *ByteView {
	return &ByteView{data: data}
}

func newByteViewFromString(data string) *ByteView {
	return NewByteView([]byte(data))
}

// Next returns a sub-slice of the next N bytes from the view, advancing the
// offset by N.
//
// This function panics if consuming N bytes would exceed the size of the
// underlying slice.
func (v *ByteView) Next(n uint16) []byte {
	if int(v.offset+n) > v.Size() {
		panic(fmt.Errorf("cannot read %d bytes (idx=%d size=%d)", n, v.offset, v.Size()))
	}
	start, end := v.offset, v.offset+n
	v.offset = end
	return v.data[start:end]
}

// NextByte returns a single byte from the underlying slice, advancing the
// offset by 1.
func (v *ByteView) NextByte() byte {
	return v.Next(1)[0]
}

// Range returns an arbitrary range of bytes from the underlying slice without
// changing the offset.
//
// This function panics if the range is invalid or would exceed the size of the
// underlying slice.
func (v *ByteView) Range(start, end uint16) []byte {
	if start > end {
		panic(fmt.Errorf("invalid range: start=%d end=%d", start, end))
	}
	if int(end) > v.Size() {
		panic(fmt.Errorf("invalid range: end=%d size=%d", end, v.Size()))
	}
	return v.data[start:end]
}

// Size returns the length of the underlying slice.
func (v *ByteView) Size() int {
	return len(v.data)
}
