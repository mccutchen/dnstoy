package weekendns

import (
	"testing"

	"github.com/carlmjohnson/be"
)

func TestHeaderToBytesExample(t *testing.T) {
	// header_to_bytes(DNSHeader(id=0x1314, flags=0, num_questions=1, num_additionals=0, num_authorities=0, num_answers=0))
	// b'\x13\x14\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00'
	header := Header{
		ID:            0x1314,
		QuestionCount: 1,
	}
	got := header.Encode()
	want := "\x13\x14\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00"
	be.Equal(t, want, string(got))
	be.Equal(t, len(got), cap(got)) // ensure we compute correct output size
}

func TestEncodeDNSNameExample(t *testing.T) {
	got := encodeName("google.com")
	want := "\x06google\x03com\x00"
	be.Equal(t, want, string(got))
	be.Equal(t, len(got), cap(got)) // ensure we compute correct output size
}

func TestEncodeQuery(t *testing.T) {
	query := newQueryHelper("google.com", QueryTypeA, 1)
	got := query.Encode()
	want := "\x00\x01\x01\x00\x00\x01\x00\x00\x00\x00\x00\x00\x06google\x03com\x00\x00\x01\x00\x01"
	be.Equal(t, want, string(got))
	be.Equal(t, len(got), cap(got)) // ensure we compute correct output size
}
