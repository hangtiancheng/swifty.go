// Package httpapi exposes the key-value store over HTTP.
package httpapi

import (
	"io"
	"net/http"
	"strconv"

	"github.com/hangtiancheng/swifty.go/apps/my-raft/internal/kvstore"
	"github.com/hangtiancheng/swifty.go/apps/my-raft/internal/raft"
)

// Service serves the key-value API and cluster membership changes over HTTP.
type Service struct {
	proposeC    chan<- string
	confChangeC chan<- raft.ConfChange
	kvStore     *kvstore.Store
}

// NewService creates an HTTP service backed by the given key-value store.
func NewService(kvStore *kvstore.Store, proposeC chan<- string, confChangeC chan<- raft.ConfChange) *Service {
	return &Service{
		proposeC:    proposeC,
		confChangeC: confChangeC,
		kvStore:     kvStore,
	}
}

// ServeHTTP handles PUT /<key> with the value as the request body, and
// POST /<nodeID> to add a node to the Raft cluster.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

// Serve starts the HTTP API server on the given port and blocks until the
// server fails.
func Serve(port int, s *Service) {
	srv := http.Server{
		Addr:    ":" + strconv.Itoa(port),
		Handler: s,
	}

	if err := srv.ListenAndServe(); err != nil {
		panic(err)
	}
}
