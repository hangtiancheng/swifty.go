package handler

import (
	"strconv"
	"strings"
)

// CRLF is the standard redis line delimiter.
const CRLF = "\r\n"

type OKReply struct{}

func NewOKReply() *OKReply {
	return theOkReply
}

var okBytes = []byte("+OK\r\n")

func (o *OKReply) ToBytes() []byte {
	return okBytes
}

var theOkReply = new(OKReply)

// SimpleStringReply. Protocol: [+][string][CRLF]
type SimpleStringReply struct {
	Str string
}

func NewSimpleStringReply(str string) *SimpleStringReply {
	return &SimpleStringReply{
		Str: str,
	}
}

func (s *SimpleStringReply) ToBytes() []byte {
	return []byte("+" + s.Str + CRLF)
}

// IntReply. Protocol: [:][int][CRLF]
type IntReply struct {
	Code int64
}

func NewIntReply(code int64) *IntReply {
	return &IntReply{
		Code: code,
	}
}

func (i *IntReply) ToBytes() []byte {
	return []byte(":" + strconv.FormatInt(i.Code, 10) + CRLF)
}

// SyntaxErrReply is a syntax-error reply.
type SyntaxErrReply struct{}

var syntaxErrBytes = []byte("-Err syntax error\r\n")
var theSyntaxErrReply = &SyntaxErrReply{}

func NewSyntaxErrReply() *SyntaxErrReply {
	return theSyntaxErrReply
}

func (r *SyntaxErrReply) ToBytes() []byte {
	return syntaxErrBytes
}

func (r *SyntaxErrReply) Error() string {
	return "Err syntax error"
}

// WrongTypeErrReply is a wrong-type error reply.
type WrongTypeErrReply struct{}

var theWrongTypeErrReply = &WrongTypeErrReply{}

var wrongTypeErrBytes = []byte("-WRONGTYPE Operation against a key holding the wrong kind of value\r\n")

func NewWrongTypeErrReply() *WrongTypeErrReply {
	return theWrongTypeErrReply
}

func (r *WrongTypeErrReply) ToBytes() []byte {
	return wrongTypeErrBytes
}

func (r *WrongTypeErrReply) Error() string {
	return "WRONGTYPE Operation against a key holding the wrong kind of value"
}

// ErrReply. Protocol: [-][err][CRLF]
type ErrReply struct {
	ErrStr string
}

func NewErrReply(errStr string) *ErrReply {
	return &ErrReply{
		ErrStr: errStr,
	}
}

func (e *ErrReply) ToBytes() []byte {
	return []byte("-" + e.ErrStr + CRLF)
}

var (
	nillReply     = &NillReply{}
	nillBulkBytes = []byte("$-1\r\n")
)

// NilReply is a global singleton. Format: [$][-1][CRLF]
type NillReply struct {
}

func NewNillReply() *NillReply {
	return nillReply
}

func (n *NillReply) ToBytes() []byte {
	return nillBulkBytes
}

// BulkReply. Protocol: [$][length][CRLF][content][CRLF]
type BulkReply struct {
	Arg []byte
}

func NewBulkReply(arg []byte) *BulkReply {
	return &BulkReply{
		Arg: arg,
	}
}

func (b *BulkReply) ToBytes() []byte {
	if b.Arg == nil {
		return nillBulkBytes
	}
	return []byte("$" + strconv.Itoa(len(b.Arg)) + CRLF + string(b.Arg) + CRLF)
}

// MultiBulkReply. Protocol: [*][length][CRLF] + length * ([$][length][CRLF][content][CRLF])
type MultiBulkReply struct {
	args [][]byte
}

func NewMultiBulkReply(args [][]byte) *MultiBulkReply {
	return &MultiBulkReply{
		args: args,
	}
}

func (m *MultiBulkReply) Args() [][]byte {
	return m.args
}

func (m *MultiBulkReply) ToBytes() []byte {
	var strBuf strings.Builder
	strBuf.WriteString("*" + strconv.Itoa(len(m.args)) + CRLF)
	for _, arg := range m.args {
		if arg == nil {
			strBuf.WriteString(string(nillBulkBytes))
			continue
		}
		strBuf.WriteString("$" + strconv.Itoa(len(arg)) + CRLF + string(arg) + CRLF)
	}
	return []byte(strBuf.String())
}

var emptyMultiBulkBytes = []byte("*0\r\n")

// EmptyMultiBulkReply is a singleton. Protocol: [*][0][CRLF]
type EmptyMultiBulkReply struct{}

func NewEmptyMultiBulkReply() *EmptyMultiBulkReply {
	return &EmptyMultiBulkReply{}
}

func (r *EmptyMultiBulkReply) ToBytes() []byte {
	return emptyMultiBulkBytes
}
