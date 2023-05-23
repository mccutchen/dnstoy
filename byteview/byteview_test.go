package byteview

import (
	"testing"

	"github.com/carlmjohnson/be"
)

func TestByteView(t *testing.T) {
	s := "0123456789"
	v := FromString(s)
	be.Equal(t, v.Size(), len(s))
	be.Equal(t, 0, v.offset)

	// okay to read 4 bytes
	{
		bs, err := v.Next(4)
		be.NilErr(t, err)
		be.DeepEqual(t, bs, []byte("0123"))
		be.Equal(t, 4, v.offset)
	}
	// okay to read next 4 bytes
	{
		bs, err := v.Next(4)
		be.NilErr(t, err)
		be.DeepEqual(t, "4567", string(bs))
		be.Equal(t, 8, v.offset)
	}
	// cannot read another 4 bytes
	{
		bs, err := v.Next(4)
		be.Nonzero(t, err)
		be.Equal(t, "EOF: cannot read 4 bytes (offset=8 size=10)", err.Error())
		// no incomplete reads: nil slice returned and internal state unchanged
		be.DeepEqual(t, nil, bs)
		be.Equal(t, 8, v.offset)
	}

	// reading single byte okay
	{
		b, err := v.NextByte()
		be.NilErr(t, err)
		be.Equal(t, []byte("8")[0], b)
		be.Equal(t, 9, v.offset)
	}
	// reading single byte okay one more time
	{
		b, err := v.NextByte()
		be.NilErr(t, err)
		be.Equal(t, []byte("9")[0], b)
		be.Equal(t, 10, v.offset)
	}
	// no more bytes to read
	{
		b, err := v.NextByte()
		be.Nonzero(t, err)
		be.Equal(t, "EOF: cannot read 1 bytes (offset=10 size=10)", err.Error())
		be.Equal(t, b, 0)
		be.Equal(t, 10, v.offset)
	}

	// new views into the same data
	{
		v2, err := v.WithOffset(5)
		be.NilErr(t, err)
		be.Equal(t, v2.offset, 5)

		bs, err := v2.Next(5)
		be.NilErr(t, err)
		be.Equal(t, "56789", string(bs))
		be.Equal(t, 10, v2.offset)
	}

	// can't create an invalid new view
	{
		v2, err := v.WithOffset(11)
		be.Nonzero(t, err)
		be.Equal(t, "EOF: invalid offset (offset=11 size=10)", err.Error())
		be.Equal(t, nil, v2)
	}

	be.Equal(t, "ByteView(offset=10, size=10)", v.String())
}
