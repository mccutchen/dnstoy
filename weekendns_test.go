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

func TestParseHeader(t *testing.T) {
	resp := newByteViewFromString("`V\x81\x80\x00\x01\x00\x01\x00\x00\x00\x00\x03www\x07example\x03com\x00\x00\x01\x00\x01\xc0\x0c\x00\x01\x00\x01\x00\x00R\x9b\x00\x04]\xb8\xd8\"")
	want := Header{
		ID:              24662,
		Flags:           33152,
		QuestionCount:   1,
		AnswerCount:     1,
		AuthorityCount:  0,
		AdditionalCount: 0,
	}
	got, err := ParseHeader(resp)
	be.NilErr(t, err)
	be.Equal(t, want, got)
}

func TestParseQuestion(t *testing.T) {
	resp := newByteViewFromString("`V\x81\x80\x00\x01\x00\x01\x00\x00\x00\x00\x03www\x07example\x03com\x00\x00\x01\x00\x01\xc0\x0c\x00\x01\x00\x01\x00\x00R\x9b\x00\x04]\xb8\xd8\"")

	// first, parse and discard the header to get it out of the way
	_, err := ParseHeader(resp)
	be.NilErr(t, err)

	want := Question{
		Name:  []byte("www.example.com"),
		Type:  QueryTypeA,
		Class: QueryClassIN,
	}
	got, err := ParseQuestion(resp)
	be.NilErr(t, err)
	be.DeepEqual(t, want, got)
}

func TestDecodeName(t *testing.T) {
	val := newByteViewFromString("\x03www\x07example\x03com\x00\x00\x01")
	got, err := decodeName(val)
	be.NilErr(t, err)
	want := "www.example.com"
	be.Equal(t, want, string(got))
}

func TestParseRecord(t *testing.T) {
	resp := newByteViewFromString("`V\x81\x80\x00\x01\x00\x01\x00\x00\x00\x00\x03www\x07example\x03com\x00\x00\x01\x00\x01\xc0\x0c\x00\x01\x00\x01\x00\x00R\x9b\x00\x04]\xb8\xd8\"")

	// parse and discard the header and question to get them out of the way
	_, err := ParseHeader(resp)
	be.NilErr(t, err)
	_, err = ParseQuestion(resp)
	be.NilErr(t, err)

	want := Record{
		Name:  []byte("www.example.com"),
		Type:  QueryTypeA,
		Class: QueryClassIN,
		TTL:   21147,
		Data:  []byte("]\xb8\xd8\""),
	}
	got, err := ParseRecord(resp)
	be.NilErr(t, err)
	be.DeepEqual(t, want, got)
}

func TestParseMessage(t *testing.T) {
	resp := newByteViewFromString("`V\x81\x80\x00\x01\x00\x01\x00\x00\x00\x00\x03www\x07example\x03com\x00\x00\x01\x00\x01\xc0\x0c\x00\x01\x00\x01\x00\x00R\x9b\x00\x04]\xb8\xd8\"")

	got, err := ParseMessage(resp)
	be.NilErr(t, err)

	want := Message{
		Header: Header{
			ID:              24662,
			Flags:           33152,
			QuestionCount:   1,
			AnswerCount:     1,
			AuthorityCount:  0,
			AdditionalCount: 0,
		},
		Questions: []Question{
			{
				Name:  []byte("www.example.com"),
				Type:  QueryTypeA,
				Class: QueryClassIN,
			},
		},
		Answers: []Record{
			{
				Name:  []byte("www.example.com"),
				Type:  QueryTypeA,
				Class: QueryClassIN,
				TTL:   21147,
				Data:  []byte("]\xb8\xd8\""),
			},
		},
		Authorities: []Record{},
		Additionals: []Record{},
	}
	be.DeepEqual(t, want, got)
}
