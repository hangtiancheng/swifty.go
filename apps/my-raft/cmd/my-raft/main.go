// Command my-raft starts a Raft-backed key-value store with an HTTP API.
package main

import (
	"github.com/hangtiancheng/swifty.go/apps/my-raft/internal/httpapi"
	"github.com/hangtiancheng/swifty.go/apps/my-raft/internal/kvstore"
	"github.com/hangtiancheng/swifty.go/apps/my-raft/internal/proxy"
	"github.com/hangtiancheng/swifty.go/apps/my-raft/internal/raft"
)

func main() {
	// Channel for submitting write requests.
	proposeC := make(chan string)
	// Channel for submitting configuration changes.
	confChangeC := make(chan raft.ConfChange)

	// Start the Raft proxy and get the channel that delivers committed data.
	commitC := proxy.New(1, []string{}, proposeC, confChangeC)
	// Create the key-value store application.
	kvStore := kvstore.New(proposeC, commitC)

	// Start the HTTP API service.
	s := httpapi.NewService(kvStore, proposeC, confChangeC)
	httpapi.Serve(8091, s)
}
