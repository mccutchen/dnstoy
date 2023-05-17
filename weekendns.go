package weekendns

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

// Question defines a DNS packet question.
type Question struct {
	Name  []byte
	Type  QueryType
	Class QueryClass
}
