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

package protocol

import (
	"bufio"
	"bytes"
	"io"
	"strconv"

	"github.com/hangtiancheng/swifty.go/demo/redis_demo/handler"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/lib/pool"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/log"
)

type lineParser func(header []byte, reader *bufio.Reader) *handler.Droplet

type Parser struct {
	lineParsers map[byte]lineParser
	logger      log.Logger
}

func NewParser(logger log.Logger) handler.Parser {
	p := Parser{
		logger: logger,
	}
	p.lineParsers = map[byte]lineParser{
		'+': p.parseSimpleString,
		'-': p.parseError,
		':': p.parseInt,
		'$': p.parseBulk,
		'*': p.parseMultiBulk,
	}
	return &p
}

func (p *Parser) ParseStream(reader io.Reader) <-chan *handler.Droplet {
	ch := make(chan *handler.Droplet)
	pool.Submit(
		func() {
			p.parse(reader, ch)
		})
	return ch
}

func (p *Parser) parse(rawReader io.Reader, ch chan<- *handler.Droplet) {
	reader := bufio.NewReader(rawReader)
	for {
		firstLine, err := reader.ReadBytes('\n')
		if err != nil {
			ch <- &handler.Droplet{
				Reply: handler.NewErrReply(err.Error()),
				Err:   err,
			}
			return
		}

		length := len(firstLine)
		if length <= 2 || firstLine[length-1] != '\n' || firstLine[length-2] != '\r' {
			continue
		}

		firstLine = bytes.TrimSuffix(firstLine, []byte{'\r', '\n'})
		lineParseFunc, ok := p.lineParsers[firstLine[0]]
		if !ok {
			p.logger.Errorf("[parser] invalid line handler: %c", firstLine[0])
			continue
		}

		ch <- lineParseFunc(firstLine, reader)
	}
}

// parseSimpleString parses a simple string reply.
func (p *Parser) parseSimpleString(header []byte, reader *bufio.Reader) *handler.Droplet {
	content := header[1:]
	return &handler.Droplet{
		Reply: handler.NewSimpleStringReply(string(content)),
	}
}

// parseInt parses an integer reply.
func (p *Parser) parseInt(header []byte, reader *bufio.Reader) *handler.Droplet {

	i, err := strconv.ParseInt(string(header[1:]), 10, 64)
	if err != nil {
		return &handler.Droplet{
			Err:   err,
			Reply: handler.NewErrReply(err.Error()),
		}
	}

	return &handler.Droplet{
		Reply: handler.NewIntReply(i),
	}
}

// parseError parses an error reply.
func (p *Parser) parseError(header []byte, reader *bufio.Reader) *handler.Droplet {
	return &handler.Droplet{
		Reply: handler.NewErrReply(string(header[1:])),
	}
}

// parseBulk parses a bulk string reply.
func (p *Parser) parseBulk(header []byte, reader *bufio.Reader) *handler.Droplet {
	// Parse the bulk string body.
	body, err := p.parseBulkBody(header, reader)
	if err != nil {
		return &handler.Droplet{
			Reply: handler.NewErrReply(err.Error()),
			Err:   err,
		}
	}
	return &handler.Droplet{
		Reply: handler.NewBulkReply(body),
	}
}

// Parse the bulk string body.
func (p *Parser) parseBulkBody(header []byte, reader *bufio.Reader) ([]byte, error) {
	// Read the string length.
	strLen, err := strconv.ParseInt(string(header[1:]), 10, 64)
	if err != nil {
		return nil, err
	}

	// Length + 2 to account for the trailing CRLF.
	body := make([]byte, strLen+2)
	// Read exactly that many bytes from the reader.
	if _, err = io.ReadFull(reader, body); err != nil {
		return nil, err
	}
	return body[:len(body)-2], nil
}

// parseMultiBulk parses a multi-bulk reply.
func (p *Parser) parseMultiBulk(header []byte, reader *bufio.Reader) (droplet *handler.Droplet) {
	var _err error
	defer func() {
		if _err != nil {
			droplet = &handler.Droplet{
				Reply: handler.NewErrReply(_err.Error()),
				Err:   _err,
			}
		}
	}()

	// Read the array length.
	length, err := strconv.ParseInt(string(header[1:]), 10, 64)
	if err != nil {
		_err = err
		return
	}

	if length <= 0 {
		return &handler.Droplet{
			Reply: handler.NewEmptyMultiBulkReply(),
		}
	}

	lines := make([][]byte, 0, length)
	for i := int64(0); i < length; i++ {
		// Read the first line of each bulk.
		firstLine, err := reader.ReadBytes('\n')
		if err != nil {
			_err = err
			return
		}

		// Validate the bulk first-line format.
		length := len(firstLine)
		if length < 4 || firstLine[length-2] != '\r' || firstLine[length-1] != '\n' || firstLine[0] != '$' {
			continue
		}

		// Parse the bulk body.
		bulkBody, err := p.parseBulkBody(firstLine[:length-2], reader)
		if err != nil {
			_err = err
			return
		}

		lines = append(lines, bulkBody)
	}

	return &handler.Droplet{
		Reply: handler.NewMultiBulkReply(lines),
	}
}
