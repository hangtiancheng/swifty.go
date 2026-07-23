package raft

type ReadState struct {
	// Commit index captured when the read request was received
	Index uint64
	// Unique identifier for the read request
	RequestCtx []byte
}

type readIndexStatus struct {
	req Message
	// Commit index at the time the read request was received
	index uint64
	// Set of nodes that acknowledged this read request
	acks map[uint64]struct{}
}

type readOnly struct {
	// Pending read requests keyed by request ID
	pendingReadIndex map[string]*readIndexStatus
	// Ordered queue of read request IDs
	readIndexQueue []string
}

func newReadOnly() *readOnly {
	return &readOnly{
		pendingReadIndex: make(map[string]*readIndexStatus),
	}
}
