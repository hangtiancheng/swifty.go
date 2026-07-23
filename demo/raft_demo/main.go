package main

import "github.com/hangtiancheng/swifty.go/demo/raft_demo/raft"

func main() {
	// Channel for write proposals
	proposeC := make(chan string)
	// Channel for configuration change proposals
	confChangeC := make(chan raft.ConfChange)

	// Create raft proxy and obtain the commit channel
	commitC := newRaftProxy(1, []string{}, proposeC, confChangeC)
	// Create the key-value store application
	kvStore := newKVStore(proposeC, commitC)

	// Start the HTTP API server
	s := newService(kvStore, proposeC, confChangeC)
	serveHTTPAPI(8091, s)
}
