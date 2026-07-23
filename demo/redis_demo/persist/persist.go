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

package persist

import (
	"context"
	"io"

	"github.com/hangtiancheng/swifty.go/demo/redis_demo/handler"
)

type Thinker interface {
	AppendOnly() bool
	AppendFileName() string
	AppendFsync() string
	AutoAofRewriteAfterCmd() int
}

func NewPersister(thinker Thinker) (handler.Persister, error) {
	if !thinker.AppendOnly() {
		return newFakePersister(nil), nil
	}

	return newAofPersister(thinker)
}

type fakeReadCloser struct {
	io.Reader
	closeCb func() error
}

func readCloserAdapter(reader io.Reader, closeCb func() error) io.ReadCloser {
	return &fakeReadCloser{Reader: reader, closeCb: closeCb}
}

func (f *fakeReadCloser) Close() error {
	return f.closeCb()
}

func newFakePersister(readCloser io.ReadCloser) handler.Persister {
	f := fakePersister{}
	if readCloser == nil {
		f.readCloser = singleFakeReloader
		return &f
	}
	f.readCloser = readCloser
	return &f
}

type fakePersister struct {
	readCloser io.ReadCloser
}

func (f *fakePersister) Reloader() (io.ReadCloser, error) {
	return f.readCloser, nil
}

func (f *fakePersister) PersistCmd(ctx context.Context, cmd [][]byte) {}

func (f *fakePersister) Close() {}

var singleFakeReloader = &fakeReloader{}

type fakeReloader struct {
}

func (f *fakeReloader) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

func (f *fakeReloader) Close() error {
	return nil
}
