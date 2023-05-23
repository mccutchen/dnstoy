package byteview

import (
	"fmt"
	"io"
)

// View is a simplistic wrapper around a slice of bytes that maintains an
// internal index into its data and exposes a vaguely io.Reader-esque API.
//
// The idea is that an incoming DNS packet will be read entirely into memory,
// and this will be an efficient way to index into it while maintaining a
// pointer to the most recently read byte.
type View struct {
	data   []byte
	offset uint16
}

// New creates a new ByteView around the given slice of bytes.
func New(data []byte) *View {
	return &View{data: data}
}

// FromString creates a new ByteView from a sting.
func FromString(data string) *View {
	return New([]byte(data))
}

// Next returns a sub-slice of the next N bytes from the view, advancing the
// offset by N.
func (v *View) Next(n uint16) ([]byte, error) {
	if int(v.offset+n) > v.Size() {
		return nil, fmt.Errorf("%w: cannot read %d bytes (offset=%d size=%d)", io.EOF, n, v.offset, v.Size())
	}
	start, end := v.offset, v.offset+n
	v.offset = end
	return v.data[start:end], nil
}

// NextByte returns a single byte from the underlying slice, advancing the
// offset by 1.
func (v *View) NextByte() (byte, error) {
	b, err := v.Next(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

// Size returns the length of the underlying slice.
func (v *View) Size() int {
	return len(v.data)
}

// WithOffset returns a ByteView with a new offset into the same underlying
// slice of bytes.
func (v *View) WithOffset(offset uint16) (*View, error) {
	if int(offset) > v.Size() {
		return nil, fmt.Errorf("%w: invalid offset (offset=%d size=%d)", io.EOF, offset, v.Size())
	}
	return &View{
		offset: offset,
		data:   v.data,
	}, nil
}

func (v *View) String() string {
	return fmt.Sprintf("ByteView(offset=%v, size=%v)", v.offset, v.Size())
}
