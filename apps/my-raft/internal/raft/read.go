package raft

type ReadState struct {
	// Index is the committed index at the time of this read request.
	Index uint64
	// RequestCtx is the unique ID of the read request.
	RequestCtx []byte
}
