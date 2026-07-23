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
