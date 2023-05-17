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
