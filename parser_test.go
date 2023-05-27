package weekendns

import (
	"errors"
	"net"
	"testing"

	"github.com/carlmjohnson/be"

	"github.com/mccutchen/weekendns/internal/byteview"
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
	query := newQueryHelper("google.com", ResourceTypeA, 1)

	// Manually set RD (Recursion Desired) flag on the header for this test,
	// since the specific test case here is from part 2, when this was added
	// to all queries. The flag was later dropped in part 3.
	// https://datatracker.ietf.org/doc/html/rfc1035#section-4.1.1
	query.Header.Flags = 1 << 8

	got := query.Encode()
	want := "\x00\x01\x01\x00\x00\x01\x00\x00\x00\x00\x00\x00\x06google\x03com\x00\x00\x01\x00\x01"
	be.Equal(t, want, string(got))
	be.Equal(t, len(got), cap(got)) // ensure we compute correct output size
}

func TestParseHeader(t *testing.T) {
	resp := byteview.FromString("`V\x81\x80\x00\x01\x00\x01\x00\x00\x00\x00\x03www\x07example\x03com\x00\x00\x01\x00\x01\xc0\x0c\x00\x01\x00\x01\x00\x00R\x9b\x00\x04]\xb8\xd8\"")
	want := Header{
		ID:              24662,
		Flags:           33152,
		QuestionCount:   1,
		AnswerCount:     1,
		AuthorityCount:  0,
		AdditionalCount: 0,
	}
	got, err := parseHeader(resp)
	be.NilErr(t, err)
	be.Equal(t, want, got)
}

func TestParseQuestion(t *testing.T) {
	resp := byteview.FromString("`V\x81\x80\x00\x01\x00\x01\x00\x00\x00\x00\x03www\x07example\x03com\x00\x00\x01\x00\x01\xc0\x0c\x00\x01\x00\x01\x00\x00R\x9b\x00\x04]\xb8\xd8\"")

	// first, parse and discard the header to get it out of the way
	_, err := parseHeader(resp)
	be.NilErr(t, err)

	want := Question{
		Name:  []byte("www.example.com"),
		Type:  ResourceTypeA,
		Class: ResourceClassIN,
	}
	got, err := parseQuestion(resp)
	be.NilErr(t, err)
	be.DeepEqual(t, want, got)
}

func TestDecodeName(t *testing.T) {
	val := byteview.FromString("\x03www\x07example\x03com\x00\x00\x01")
	got, err := decodeName(val)
	be.NilErr(t, err)
	want := "www.example.com"
	be.Equal(t, want, string(got))
}

func TestParseRecord(t *testing.T) {
	resp := byteview.FromString("`V\x81\x80\x00\x01\x00\x01\x00\x00\x00\x00\x03www\x07example\x03com\x00\x00\x01\x00\x01\xc0\x0c\x00\x01\x00\x01\x00\x00R\x9b\x00\x04]\xb8\xd8\"")

	// parse and discard the header and question to get them out of the way
	_, err := parseHeader(resp)
	be.NilErr(t, err)
	_, err = parseQuestion(resp)
	be.NilErr(t, err)

	want := Record{
		Name:  []byte("www.example.com"),
		Type:  ResourceTypeA,
		Class: ResourceClassIN,
		TTL:   21147,
		Data:  []byte("]\xb8\xd8\""),
	}
	got, err := parseRecord(resp)
	be.NilErr(t, err)
	be.DeepEqual(t, want, got)
}

func TestParseMessage(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		resp string
		want Message
	}{
		"example.com input from exercise part 2": {
			resp: "`V\x81\x80\x00\x01\x00\x01\x00\x00\x00\x00\x03www\x07example\x03com\x00\x00\x01\x00\x01\xc0\x0c\x00\x01\x00\x01\x00\x00R\x9b\x00\x04]\xb8\xd8\"",
			want: Message{
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
						Type:  ResourceTypeA,
						Class: ResourceClassIN,
					},
				},
				Answers: []Record{
					{
						Name:  []byte("www.example.com"),
						Type:  ResourceTypeA,
						Class: ResourceClassIN,
						TTL:   21147,
						Data:  []byte("]\xb8\xd8\""),
					},
				},
				Authorities: []Record{},
				Additionals: []Record{},
			},
		},
		"actual www.facebook.com response": {
			resp: "\x8bX\x81\x80\x00\x01\x00\x02\x00\x00\x00\x00\x03www\bfacebook\x03com\x00\x00\x01\x00\x01\xc0\f\x00\x05\x00\x01\x00\x00\f\x0e\x00\x11\tstar-mini\x04c10r\xc0\x10\xc0.\x00\x01\x00\x01\x00\x00\x00\x11\x00\x04\x9d\xf0\xf1#",
			want: Message{
				Header: Header{
					ID:            0x8b58,
					Flags:         0x8180,
					QuestionCount: 1,
					AnswerCount:   2,
				},
				Questions: []Question{
					{
						Name:  []byte("www.facebook.com"),
						Type:  ResourceTypeA,
						Class: ResourceClassIN,
					},
				},
				Answers: []Record{
					{
						Name:  []byte("www.facebook.com"),
						Type:  0x5,
						Class: ResourceClassIN,
						TTL:   3086,
						Data:  []byte("\tstar-mini\x04c10r\xc0\x10"),
					},
					{
						Name:  []byte("star-mini.c10r.facebook.com"),
						Type:  ResourceTypeA,
						Class: ResourceClassIN,
						TTL:   17,
						Data:  []byte("\x9d\xf0\xf1#"),
					},
				},
				Authorities: []Record{},
				Additionals: []Record{},
			},
		},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := parseMessage(byteview.FromString(tc.resp))
			be.NilErr(t, err)
			be.DeepEqual(t, tc.want, got)
		})
	}
}

func TestParseIPs(t *testing.T) {
	testCases := []struct {
		ipData  []byte
		want    []net.IP
		wantErr error
	}{
		{
			ipData: []byte{93, 184, 216, 34},
			want:   []net.IP{net.IPv4(93, 184, 216, 34)},
		},
		{
			ipData: []byte{
				93, 184, 216, 34,
				1, 2, 3, 4,
				5, 6, 7, 8,
			},
			want: []net.IP{
				net.IPv4(93, 184, 216, 34),
				net.IPv4(1, 2, 3, 4),
				net.IPv4(5, 6, 7, 8),
			},
		},
		{
			// not enough data
			ipData:  []byte{1},
			wantErr: errors.New(`parseIP: invalid IP address data: "\x01"`),
		},
		{
			// not evenly divisible by 4
			ipData:  []byte{1, 2, 3, 4, 5},
			wantErr: errors.New(`parseIP: invalid IP address data: "\x01\x02\x03\x04\x05"`),
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(string(tc.ipData), func(t *testing.T) {
			got, err := parseIPAddrs(tc.ipData)
			if tc.wantErr != nil {
				be.Nonzero(t, err)
				be.Equal(t, tc.wantErr.Error(), err.Error())
				return
			}
			be.NilErr(t, err)
			be.DeepEqual(t, tc.want, got)
		})
	}
}
