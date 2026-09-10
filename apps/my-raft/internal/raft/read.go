package raft

type ReadState struct {
	// Index is the committed index at the time of this read request.
	Index uint64
	// RequestCtx is the unique ID of the read request.
	RequestCtx []byte
}

type readIndexStatus struct {
	req Message
	// Commit index at the time the read request was received.
	index uint64
	// Records which nodes have responded to this read request.
	acks map[uint64]struct{}
}

type readOnly struct {
	// Read requests that are not finished yet; key: request ID, value: request state.
	pendingReadIndex map[string]*readIndexStatus
	// FIFO queue of request IDs recording the order of the read requests.
	readIndexQueue []string
}

func newReadOnly() *readOnly {
	return &readOnly{
		pendingReadIndex: make(map[string]*readIndexStatus),
	}
}
