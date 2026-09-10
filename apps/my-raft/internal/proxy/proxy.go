// Package proxy wires the local application to a Raft node: it feeds
// proposals and configuration changes into the node and publishes committed
// entries back to the application.
package proxy

import (
	"context"
	"time"

	"github.com/hangtiancheng/swifty.go/apps/my-raft/internal/raft"
)

type raftProxy struct {
	// Channel on which users submit write requests.
	proposeC <-chan string
	// Channel on which users submit configuration change requests.
	confChangeC <-chan raft.ConfChange
	// Channel that delivers committed entries.
	commitC chan<- *string
	// ID of this node.
	id uint64
	// List of peer addresses.
	peers []string

	// The Raft node.
	node raft.Node

	// Log persistence module.
	storage raft.Storage
}

// New starts a Raft node for the given id and peers and returns the channel
// on which committed entries are delivered.
func New(id uint64, peers []string, proposeC <-chan string, confChangeC <-chan raft.ConfChange) <-chan *string {
	commitC := make(chan *string)
	r := raftProxy{
		proposeC:    proposeC,
		confChangeC: confChangeC,
		commitC:     commitC,
		id:          id,
		peers:       peers,
		storage:     raft.NewMemoryStorage(),
	}

	go r.run()
	return commitC
}

func (r *raftProxy) run() {
	peers := make([]raft.Peer, 0, len(r.peers))
	for i := range r.peers {
		peers = append(peers, raft.Peer{ID: uint64(i + 1)})
	}

	c := raft.Config{
		ID:            r.id,
		ElectionTick:  10,
		HeartbeatTick: 1,
		Storage:       r.storage,
	}

	r.node = raft.StartNode(&c, peers)

	// The transport module starts here.

	// The listener starts below.
	go r.listen()
}

func (r *raftProxy) listen() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Listen on the two channels for client-submitted requests.
	go r.listenRequest()
	// Main loop: consume Ready.
	for {
		select {
		case <-ticker.C:
			r.node.Tick()

		case <-r.node.Ready():
			// Persist the hard state and the configuration.

			// Persist entries.

			// Send messages.

			// Apply committed entries.

			// Advance.
			r.node.Advance()
		}
	}
}

func (r *raftProxy) listenRequest() {
	for {
		select {
		case prop, ok := <-r.proposeC:
			if !ok {
				return
			}
			r.node.Propose(context.Background(), []byte(prop))

		case cc, ok := <-r.confChangeC:
			if !ok {
				return
			}
			r.node.ProposeConfChange(context.Background(), cc)
		}
	}

}
