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
