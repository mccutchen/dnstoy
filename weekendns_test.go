package weekendns

import (
	"encoding/hex"
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
	want := []byte{0x13, 0x14, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
	compareByteSlices(t, want, got)
	be.Equal(t, len(got), cap(got))
}

func TestEncodeDNSNameExample(t *testing.T) {
	// encode_dns_name("google.com")
	// b'\x06google\x03com\x00'
	got := encodeName("google.com")
	want := []byte{}
	want = append(want, 0x6)
	want = append(want, []byte("google")...)
	want = append(want, 0x3)
	want = append(want, []byte("com")...)
	want = append(want, 0x0)
	compareByteSlices(t, want, got)
	be.Equal(t, len(got), cap(got))
}

func TestEncodeQuery(t *testing.T) {
	query := newQueryHelper("google.com", QueryTypeA, 1)
	got := query.Encode()
	want := []byte{0x0, 0x1, 0x1, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x6, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x3, 0x63, 0x6f, 0x6d, 0x0, 0x0, 0x1, 0x0, 0x1}
	compareByteSlices(t, want, got)
}

func compareByteSlices(t *testing.T, want []byte, got []byte) {
	t.Helper()
	wantHex := hex.EncodeToString(want)
	gotHex := hex.EncodeToString(got)
	t.Logf("want bytes: %#v", want)
	t.Logf("got  bytes: %#v", got)
	t.Logf("want hex:   %q", wantHex)
	t.Logf("got  hex:   %q", gotHex)
	be.Equal(t, wantHex, gotHex)
}
