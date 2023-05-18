package weekendns

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/rand"
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
)

type Reader interface {
	io.ByteReader
	io.Reader
	io.ReaderAt
}

// Header defines a DNS packet header.
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

// parseHeader parses a DNS header packet from a reader.
func parseHeader(r Reader) (Header, error) {
	buf := make([]byte, 12)
	if _, err := io.ReadFull(r, buf); err != nil {
		return Header{}, err
	}
	return Header{
		ID:              binary.BigEndian.Uint16(buf[:2]),
		Flags:           binary.BigEndian.Uint16(buf[2:4]),
		QuestionCount:   binary.BigEndian.Uint16(buf[4:6]),
		AnswerCount:     binary.BigEndian.Uint16(buf[6:8]),
		AuthorityCount:  binary.BigEndian.Uint16(buf[8:10]),
		AdditionalCount: binary.BigEndian.Uint16(buf[10:12]),
	}, nil
}

// Question defines a DNS packet question.
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

// parseQuestion parses a DNS question packet from a reader.
func parseQuestion(r Reader) (Question, error) {
	name, err := decodeNameSimple(r)
	if err != nil {
		return Question{}, fmt.Errorf("parseQuestion: error decoding name: %w", err)
	}
	buf := make([]byte, 4)
	if n, err := io.ReadFull(r, buf); err != nil {
		return Question{}, fmt.Errorf("parseQuestion: error reading fields: %w (read %d/%d bytes)", err, n, len(buf))
	}
	return Question{
		Name:  name,
		Type:  QueryType(binary.BigEndian.Uint16(buf[:2])),
		Class: QueryClass(binary.BigEndian.Uint16(buf[2:4])),
	}, nil
}

// Record defines a DNS packet record.
type Record struct {
	Name  []byte
	Type  QueryType
	Class QueryClass
	TTL   uint32 // ???
	Data  []byte
}

// parseRecord parses a DNS record packet from a reader.
func parseRecord(r Reader) (Record, error) {
	name, err := decodeNameSimple(r)
	if err != nil {
		return Record{}, fmt.Errorf("parseRecord: error decoding name: %w", err)
	}

	// the type, class, TTL, and data length together are 10 bytes (2 + 2 + 4 + 2 = 10)
	// so we read 10 bytes
	buf := make([]byte, 10)
	if n, err := io.ReadFull(r, buf); err != nil {
		return Record{}, fmt.Errorf("parseRecord: error reading fields: %w (read %d/%d bytes)", err, n, len(buf))
	}

	dataLen := binary.BigEndian.Uint16(buf[8:10])
	data := make([]byte, dataLen)
	if n, err := io.ReadFull(r, data); err != nil {
		return Record{}, fmt.Errorf("parseRecord: error reading data: %w (read %d/%d bytes)", err, n, len(data))
	}

	return Record{
		Name:  name,
		Type:  QueryType(binary.BigEndian.Uint16(buf[:2])),
		Class: QueryClass(binary.BigEndian.Uint16(buf[2:4])),
		TTL:   binary.BigEndian.Uint32(buf[4:8]),
		Data:  data,
	}, nil
}

// Query defines a DNS query.
type Query struct {
	Header   Header
	Question Question
}

// NewQuery creates a new DNS query for the given domain name and record type.
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

// decodeNameSimple decodes a DNS name. See encodeName for details on the
// format.
func decodeNameSimple(r Reader) ([]byte, error) {
	out := &bytes.Buffer{}
	for i := 0; ; i++ {
		length, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		if length == 0 {
			break
		}

		// only write the "." separator after first iteration
		if i > 0 {
			out.Write([]byte("."))
		}

		if _, err := io.CopyN(out, r, int64(length)); err != nil {
			return nil, err
		}
	}
	return out.Bytes(), nil
}
