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
)

var loadingPersisterPattern int
var ctxKeyLoadingPersisterPattern = &loadingPersisterPattern

func SetLoadingPattern(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeyLoadingPersisterPattern, true)
}

func IsLoadingPattern(ctx context.Context) bool {
	is, _ := ctx.Value(ctxKeyLoadingPersisterPattern).(bool)
	return is
}

type Persister interface {
	Reloader() (io.ReadCloser, error)
	PersistCmd(ctx context.Context, cmd [][]byte)
	Close()
}

type fakeReadWriter struct {
	io.Reader
}

func newFakeReaderWriter(reader io.Reader) io.ReadWriter {
	return &fakeReadWriter{
		Reader: reader,
	}
}

func (f *fakeReadWriter) Write(p []byte) (n int, err error) {
	// log ...
	return 0, nil
}
