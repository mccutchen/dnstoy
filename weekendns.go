package weekendns

import (
	"encoding/binary"
)

type (
	QueryType  uint16
	QueryClass uint16
)

const (
	QueryTypeA   = 1
	QueryClassIN = 1
)

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
