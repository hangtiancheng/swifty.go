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

package main

import (
	"io"
	"net/http"
	"strconv"

	"github.com/hangtiancheng/swifty.go/demo/raft_demo/raft"
)

type service struct {
	proposeC    chan<- string
	confChangeC chan<- raft.ConfChange
	kvStore     *kvStore
}

func newService(kvStore *kvStore, proposeC chan<- string, confChangeC chan<- raft.ConfChange) *service {
	return &service{
		proposeC:    proposeC,
		confChangeC: confChangeC,
		kvStore:     kvStore,
	}
}

func (s *service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	url := r.RequestURI

	switch {
	case r.Method == http.MethodPut:
		v, err := io.ReadAll(r.Body)
		if err != nil {
			panic(err)
		}
		s.kvStore.Propose(url, string(v))

	case r.Method == http.MethodPost:
		v, err := io.ReadAll(r.Body)
		if err != nil {
			panic(err)
		}

		nodeID, err := strconv.ParseUint(url[1:], 0, 64)
		if err != nil {
			panic(err)
		}
		s.confChangeC <- raft.ConfChange{
			NodeID:  nodeID,
			Type:    raft.ConfChangeAddNode,
			Context: v,
		}

	}

}

func serveHttpApi(port int, s *service) {
	srv := http.Server{
		Addr:    ":" + strconv.Itoa(port),
		Handler: s,
	}

	if err := srv.ListenAndServe(); err != nil {
		panic(err)
	}
}
