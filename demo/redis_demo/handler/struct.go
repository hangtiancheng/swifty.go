// Copyright (c) 2026 hangtiancheng
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package handler

import (
	"context"
	"io"
	"strings"
)

var UnknownErrReplyBytes = []byte("-ERR unknown\r\n")

type Reply interface {
	ToBytes() []byte
}

type MultiReply interface {
	Reply
	Args() [][]byte
}

type Droplet struct {
	Reply Reply
	Err   error
}

func (d *Droplet) Terminated() bool {
	if d.Err == io.EOF || d.Err == io.ErrUnexpectedEOF {
		return true
	}
	return d.Err != nil && strings.Contains(d.Err.Error(), "use of closed network connection")
}

type DB interface {
	Do(ctx context.Context, cmdLine [][]byte) Reply
	Close()
}

// Parser is the protocol parser.
type Parser interface {
	ParseStream(reader io.Reader) <-chan *Droplet
}
