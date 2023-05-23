package weekendns

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"strconv"
	"strings"

	"github.com/mccutchen/weekendns/byteview"
)

// ResourceType represents the TYPE field in a resource record:
type ResourceType uint16

// Query types:
// https://datatracker.ietf.org/doc/html/rfc1035#section-3.2.2
const (
	ResourceTypeA     ResourceType = 1
	ResourceTypeNS    ResourceType = 2
	ResourceTypeCNAME ResourceType = 5
	ResourceTypeTXT   ResourceType = 16
)

// ResourceClass represents the CLASS field in a resource record:
type ResourceClass uint16

// Query classes:
// https://datatracker.ietf.org/doc/html/rfc1035#section-3.2.4
const (
	ResourceClassIN ResourceClass = 1
)

// "Messages carried by UDP are restricted to 512 bytes (not counting the IP or
// UDP headers)."
// https://datatracker.ietf.org/doc/html/rfc1035#section-4.2.1
const maxMessageSize = 512

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

// parseHeader parses a Header section from a slice of bytes.
func parseHeader(v *byteview.View) (Header, error) {
	bs, err := v.Next(12) // 12 == 2 bytes for each of the 6 header fields
	if err != nil {
		return Header{}, err
	}
	return Header{
		ID:              binary.BigEndian.Uint16(bs[0:2]),
		Flags:           binary.BigEndian.Uint16(bs[2:4]),
		QuestionCount:   binary.BigEndian.Uint16(bs[4:6]),
		AnswerCount:     binary.BigEndian.Uint16(bs[6:8]),
		AuthorityCount:  binary.BigEndian.Uint16(bs[8:10]),
		AdditionalCount: binary.BigEndian.Uint16(bs[10:12]),
	}, nil
}

// Question defines the Question section of a DNS message:
// https://datatracker.ietf.org/doc/html/rfc1035#section-4.1.2
type Question struct {
	Name  []byte
	Type  ResourceType
	Class ResourceClass
}

// Encode encodes a Question as bytes in network order.
func (q Question) Encode() []byte {
	out := make([]byte, 0, len(q.Name)+4) // 4 == 2 bytes each for type and class
	out = append(out, q.Name...)
	out = binary.BigEndian.AppendUint16(out, uint16(q.Type))
	out = binary.BigEndian.AppendUint16(out, uint16(q.Class))
	return out
}

// parseQuestion parses a Question section from a slice of bytes.
func parseQuestion(v *byteview.View) (Question, error) {
	name, err := decodeName(v)
	if err != nil {
		return Question{}, fmt.Errorf("parseQuestion: error decoding name: %w", err)
	}
	bs, err := v.Next(4) // 4 == 2 bytes each for type and class
	if err != nil {
		return Question{}, fmt.Errorf("parseQuestion: %w", err)
	}
	return Question{
		Name:  name,
		Type:  ResourceType(binary.BigEndian.Uint16(bs[0:2])),
		Class: ResourceClass(binary.BigEndian.Uint16(bs[2:4])),
	}, nil
}

// Record defines a Resource Record section:
// https://datatracker.ietf.org/doc/html/rfc1035#section-4.1.3
type Record struct {
	Name  []byte
	Type  ResourceType
	Class ResourceClass
	TTL   uint32
	Data  []byte
}

// parseRecord parses a DNS record section from a slice of bytes.
func parseRecord(v *byteview.View) (Record, error) {
	name, err := decodeName(v)
	if err != nil {
		return Record{}, fmt.Errorf("parseRecord: error decoding name: %w", err)
	}

	bs, err := v.Next(10) // 10 == 2 bytes each for type, class, data length and 4 bytes for TTL
	if err != nil {
		return Record{}, fmt.Errorf("parseRecord: error reading metadata: %w", err)
	}

	record := Record{
		Name:  name,
		Type:  ResourceType(binary.BigEndian.Uint16(bs[0:2])),
		Class: ResourceClass(binary.BigEndian.Uint16(bs[2:4])),
		TTL:   binary.BigEndian.Uint32(bs[4:8]),
	}

	dataLen := binary.BigEndian.Uint16(bs[8:10])

	switch record.Type {
	case ResourceTypeNS:
		// https://datatracker.ietf.org/doc/html/rfc1035#section-3.3.11
		data, err := decodeName(v)
		if err != nil {
			return record, fmt.Errorf("parseRecord: error decoding data for NS record: %w", err)
		}
		record.Data = data
	default:
		data, err := v.Next(dataLen)
		if err != nil {
			return record, fmt.Errorf("parseRecord: error reading data field: %w", err)
		}
		record.Data = data
	}

	return record, nil
}

// Query defines a DNS query message.
type Query struct {
	Header   Header
	Question Question
}

// NewQuery creates a new DNS query message for the given domain name and
// record type.
func NewQuery(domainName string, queryType ResourceType) Query {
	return newQueryHelper(domainName, queryType, uint16(rand.Intn(math.MaxUint16+1)))
}

// newQueryHelper creates a new DNS query with a given ID, used for
// deterministic testing of query building.
func newQueryHelper(domainName string, queryType ResourceType, id uint16) Query {
	return Query{
		Header: Header{
			ID:            id,
			QuestionCount: 1,
		},
		Question: Question{
			Name:  encodeName(domainName),
			Type:  queryType,
			Class: ResourceClassIN,
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

func sendQuery(dst string, domainName string, queryType ResourceType) (Message, error) {
	conn, err := net.Dial("udp", net.JoinHostPort(dst, "53"))
	if err != nil {
		return Message{}, err
	}

	query := NewQuery(domainName, queryType)
	if _, err := conn.Write(query.Encode()); err != nil {
		return Message{}, err
	}

	buf := make([]byte, maxMessageSize)
	n, err := conn.Read(buf)
	if err != nil {
		return Message{}, err
	}

	msg, err := parseMessage(byteview.New(buf[:n]))
	if err != nil {
		log.Printf("error parsing message: %s", err)
		log.Printf("response: %q", string(buf[:n]))
		return Message{}, err
	}

	return msg, nil
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

func parseMessage(v *byteview.View) (Message, error) {
	header, err := parseHeader(v)
	if err != nil {
		return Message{}, err
	}

	questions := make([]Question, header.QuestionCount)
	for i := 0; i < int(header.QuestionCount); i++ {
		question, err := parseQuestion(v)
		if err != nil {
			return Message{}, err
		}
		questions[i] = question
	}

	answers := make([]Record, header.AnswerCount)
	for i := 0; i < int(header.AnswerCount); i++ {
		rec, err := parseRecord(v)
		if err != nil {
			return Message{}, err
		}
		answers[i] = rec
	}

	authorities := make([]Record, header.AuthorityCount)
	for i := 0; i < int(header.AuthorityCount); i++ {
		rec, err := parseRecord(v)
		if err != nil {
			return Message{}, err
		}
		authorities[i] = rec
	}

	additionals := make([]Record, header.AdditionalCount)
	for i := 0; i < int(header.AdditionalCount); i++ {
		rec, err := parseRecord(v)
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
func decodeName(v *byteview.View) ([]byte, error) {
	var parts [][]byte
	for {
		length, err := v.NextByte()
		if err != nil {
			return nil, fmt.Errorf("decodeName: error reading length: %s", err)
		}

		// we're done decoding this name
		if length == 0 {
			break
		}

		// for compressed names, we need to decode the pointer to an earlier
		// offset in the same message where the name can be found.
		if isCompressed, pointerOffset, err := checkNameCompression(length, v); err != nil {
			return nil, fmt.Errorf("decodeName: error checking for name compression: %w", err)
		} else if isCompressed {
			v2, err := v.WithOffset(pointerOffset)
			if err != nil {
				return nil, fmt.Errorf("decodeName: invalid pointer offset %v: %w", pointerOffset, err)
			}
			part, err := decodeName(v2)
			if err != nil {
				return nil, fmt.Errorf("decodeName: error decoding compressed name at offset %v: %w", pointerOffset, err)
			}
			parts = append(parts, part)
			break
		} else {
			part, err := v.Next(uint16(length))
			if err != nil {
				return nil, fmt.Errorf("decodeName: error reading name part: %w", err)
			}
			parts = append(parts, part)
		}
	}
	return bytes.Join(parts, []byte(".")), nil
}

// checkNameCompression checks whether the given length indicates that name
// compression is being used. If so, another byte is read from the view in
// order to compute the offset where the referenced name can be found.
func checkNameCompression(length byte, v *byteview.View) (isCompressed bool, pointerOffset uint16, err error) {
	if length&0b1100_0000 != 0 {
		// https://datatracker.ietf.org/doc/html/rfc1035#section-4.1.4
		b, err := v.NextByte()
		if err != nil {
			return false, 0, err
		}
		pointerOffset = binary.BigEndian.Uint16([]byte{byte(length & 0b0011_1111), b})
		return true, pointerOffset, nil
	}
	return false, 0, nil
}

// formatIP formats a byte slice as a dotted decimal IP address.
func formatIP(ipData []byte) string {
	s := ""
	for i, b := range ipData {
		if i > 0 {
			s += "."
		}
		s += strconv.Itoa(int(b))
	}
	return s
}

func getAnswer(msg Message) string {
	for _, a := range msg.Answers {
		if a.Type == ResourceTypeA {
			return formatIP(a.Data)
		}
	}
	return ""
}

func getNameserverIP(msg Message) string {
	for _, a := range msg.Additionals {
		if a.Type == ResourceTypeA {
			return formatIP(a.Data)
		}
	}
	return ""
}

func getNameserverDomain(msg Message) string {
	for _, a := range msg.Authorities {
		if a.Type == ResourceTypeNS {
			return string(a.Data)
		}
	}
	return ""
}

// Resolve recursively resolves the given domain name, returning the resolved
// IP address, the parsed DNS Message, and an error.
func Resolve(domainName string, queryType ResourceType) (string, Message, error) {
	nameserver := "198.41.0.4"
	for {
		log.Printf("querying nameserver %q for domain %q", nameserver, domainName)
		msg, err := sendQuery(nameserver, domainName, queryType)
		if err != nil {
			return "", Message{}, err
		}

		// successfully resolved IP address, we're done
		if ip := getAnswer(msg); ip != "" {
			return ip, msg, nil
		}

		// recurse with new nameserver IP from the response
		if nsIP := getNameserverIP(msg); nsIP != "" {
			nameserver = nsIP
			continue
		}

		// first resolve nameserver domain to nameserver IP, then recurse with
		// new nameserver IP
		if nsDomain := getNameserverDomain(msg); nsDomain != "" {
			nextNameserver, _, err := Resolve(nsDomain, ResourceTypeA)
			if err != nil {
				return "", msg, err
			}
			nameserver = nextNameserver
			continue
		}

		return "", msg, errors.New("something went wrong")
	}
}
