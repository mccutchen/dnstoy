package weekendns

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/rand"
	"strconv"
	"strings"
)

type (
	QueryType  uint16
	QueryClass uint16
)

const (
	QueryTypeA   = 1
	QueryClassIN = 1

	FlagRecursionDesired = 1 << 8

	NameCompressionFlag = 0b1100_0000
	NameCompressionMask = 0b0011_1111
)

type Reader interface {
	io.ByteReader
	io.Reader
	io.ReaderAt
}

// Header defines the Header section of a DNS message:
// https://datatracker.ietf.org/doc/html/rfc1035#section-4.1.1
type Header struct {
	ID              uint16
	Flags           uint16
	QuestionCount   uint16
	AnswerCount     uint16
	AuthorityCount  uint16
	AdditionalCount uint16
}

// Encode encodes a Header as bytes in network order.
func (h Header) Encode() []byte {
	out := make([]byte, 0, 12) // 6 fields x 2 bytes per field
	out = binary.BigEndian.AppendUint16(out, h.ID)
	out = binary.BigEndian.AppendUint16(out, h.Flags)
	out = binary.BigEndian.AppendUint16(out, h.QuestionCount)
	out = binary.BigEndian.AppendUint16(out, h.AnswerCount)
	out = binary.BigEndian.AppendUint16(out, h.AuthorityCount)
	out = binary.BigEndian.AppendUint16(out, h.AdditionalCount)
	return out
}

// ParseHeader parses a Header section from a slice of bytes.
func ParseHeader(v *ByteView) (Header, error) {
	return Header{
		ID:              binary.BigEndian.Uint16(v.Next(2)),
		Flags:           binary.BigEndian.Uint16(v.Next(2)),
		QuestionCount:   binary.BigEndian.Uint16(v.Next(2)),
		AnswerCount:     binary.BigEndian.Uint16(v.Next(2)),
		AuthorityCount:  binary.BigEndian.Uint16(v.Next(2)),
		AdditionalCount: binary.BigEndian.Uint16(v.Next(2)),
	}, nil
}

// Question defines the Question section of a DNS message:
// https://datatracker.ietf.org/doc/html/rfc1035#section-4.1.2
type Question struct {
	Name  []byte
	Type  QueryType
	Class QueryClass
}

// Encode encodes a Question as bytes in network order.
func (q Question) Encode() []byte {
	out := make([]byte, 0, len(q.Name)+4) // 4 == 2 bytes each for type and class
	out = append(out, q.Name...)
	out = binary.BigEndian.AppendUint16(out, uint16(q.Type))
	out = binary.BigEndian.AppendUint16(out, uint16(q.Class))
	return out
}

// ParseQuestion parses a Question section from a slice of bytes.
func ParseQuestion(v *ByteView) (Question, error) {
	name, err := decodeName(v)
	if err != nil {
		return Question{}, fmt.Errorf("parseQuestion: error decoding name: %w", err)
	}
	return Question{
		Name:  name,
		Type:  QueryType(binary.BigEndian.Uint16(v.Next(2))),
		Class: QueryClass(binary.BigEndian.Uint16(v.Next(2))),
	}, nil
}

// Record defines a Resource Record section:
// https://datatracker.ietf.org/doc/html/rfc1035#section-4.1.3
type Record struct {
	Name  []byte
	Type  QueryType
	Class QueryClass
	TTL   uint32
	Data  []byte
}

// ParseRecord parses a DNS record packet from a reader.
func ParseRecord(v *ByteView) (Record, error) {
	name, err := decodeName(v)
	if err != nil {
		return Record{}, fmt.Errorf("parseRecord: error decoding name: %w", err)
	}

	record := Record{
		Name:  name,
		Type:  QueryType(binary.BigEndian.Uint16(v.Next(2))),
		Class: QueryClass(binary.BigEndian.Uint16(v.Next(2))),
		TTL:   binary.BigEndian.Uint32(v.Next(4)),
	}

	dataLen := binary.BigEndian.Uint16(v.Next(2))
	record.Data = v.Next(dataLen)
	return record, nil
}

// Query defines a DNS query message.
type Query struct {
	Header   Header
	Question Question
}

// NewQuery creates a new DNS query message for the given domain name and
// record type.
func NewQuery(domainName string, queryType QueryType) Query {
	return newQueryHelper(domainName, queryType, uint16(rand.Intn(math.MaxUint16+1)))
}

// newQueryHelper creates a new DNS query with a given ID, used for
// deterministic testing of query building.
func newQueryHelper(domainName string, queryType QueryType, id uint16) Query {
	return Query{
		Header: Header{
			ID:            id,
			QuestionCount: 1,
			Flags:         FlagRecursionDesired,
		},
		Question: Question{
			Name:  encodeName(domainName),
			Type:  queryType,
			Class: QueryClassIN,
		},
	}
}

// Encode encodes a DNS query as bytes in network order.
func (q Query) Encode() []byte {
	headerBytes := q.Header.Encode()
	questionBytes := q.Question.Encode()
	out := make([]byte, 0, len(headerBytes)+len(questionBytes))
	out = append(out, headerBytes...)
	out = append(out, questionBytes...)
	return out
}

// Message defines a DNS Message:
// https://datatracker.ietf.org/doc/html/rfc1035#section-4.1
type Message struct {
	Header      Header
	Questions   []Question
	Answers     []Record
	Authorities []Record
	Additionals []Record
}

func ParseMessage(v *ByteView) (Message, error) {
	header, err := ParseHeader(v)
	if err != nil {
		return Message{}, err
	}

	questions := make([]Question, header.QuestionCount)
	for i := 0; i < int(header.QuestionCount); i++ {
		question, err := ParseQuestion(v)
		if err != nil {
			return Message{}, err
		}
		questions[i] = question
	}

	answers := make([]Record, header.AnswerCount)
	for i := 0; i < int(header.AnswerCount); i++ {
		rec, err := ParseRecord(v)
		if err != nil {
			return Message{}, err
		}
		answers[i] = rec
	}

	authorities := make([]Record, header.AuthorityCount)
	for i := 0; i < int(header.AuthorityCount); i++ {
		rec, err := ParseRecord(v)
		if err != nil {
			return Message{}, err
		}
		authorities[i] = rec
	}

	additionals := make([]Record, header.AdditionalCount)
	for i := 0; i < int(header.AdditionalCount); i++ {
		rec, err := ParseRecord(v)
		if err != nil {
			return Message{}, err
		}
		additionals[i] = rec
	}

	return Message{
		Header:      header,
		Questions:   questions,
		Answers:     answers,
		Authorities: authorities,
		Additionals: additionals,
	}, nil
}

// encodeName encodes a DNS name by splitting it into parts and prefixing each
// part with its length and appending a nul byte, so "google.com" is encoded as
// "6 google 3 com 0".
func encodeName(name string) []byte {
	parts := strings.Split(name, ".")
	result := make([]byte, 0, len(name)+len(parts))
	for _, part := range parts {
		result = append(result, byte(len(part)))
		result = append(result, []byte(part)...)
	}
	result = append(result, 0x0)
	return result
}

// decodeName decodes a DNS name, optionally handling compression.
func decodeName(v *ByteView) ([]byte, error) {
	var parts [][]byte
	for {
		length := v.NextByte()
		if length == 0 {
			break
		}

		// for compressed names, we need to decode the pointer to an earlier
		// offset in the same message where the name can be found.
		//
		// https://datatracker.ietf.org/doc/html/rfc1035#section-4.1.4
		if length&NameCompressionFlag != 0 {
			pointerOffset := binary.BigEndian.Uint16([]byte{
				byte(length & NameCompressionMask),
				v.NextByte(),
			})
			part, err := decodeName(v.WithOffset(pointerOffset))
			if err != nil {
				return nil, fmt.Errorf("decodeName: error decoding compressed name at offset %v: %w", pointerOffset, err)
			}
			parts = append(parts, part)
			break
		}

		// otherwise, we have just grab the next length bytes for this part of
		// the name.
		parts = append(parts, v.Next(uint16(length)))
	}
	return bytes.Join(parts, []byte(".")), nil
}

func FormatIP(ipData []byte) string {
	s := ""
	for i, b := range ipData {
		if i > 0 {
			s += "."
		}
		s += strconv.Itoa(int(b))
	}
	return s
}
