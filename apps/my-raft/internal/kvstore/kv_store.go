// Package kvstore implements the in-memory key-value store that applies
// committed Raft entries to a local state machine.
package kvstore

import (
	"encoding/json"
	"log"
	"sync"
)

// Store is an in-memory key-value store backed by a Raft log.
type Store struct {
	proposeC chan<- string
	sync.RWMutex
	core map[string]string
}

type kv struct {
	Key string `json:"key"`
	Val string `json:"val"`
}

// New creates a Store and starts a goroutine that applies the entries
// delivered on commitC to the local state machine.
func New(proposeC chan<- string, commitC <-chan *string) *Store {
	s := Store{
		proposeC: proposeC,
		core:     make(map[string]string),
	}

	go s.readCommit(commitC)
	return &s
}

func (s *Store) readCommit(commitC <-chan *string) {
	for data := range commitC {
		// Apply the data to the state machine.
		var entry kv
		if err := json.Unmarshal([]byte(*data), &entry); err != nil {
			log.Printf("kvstore: skipping invalid committed entry %q: %v", *data, err)
			continue
		}

		s.Lock()
		s.core[entry.Key] = entry.Val
		s.Unlock()
	}
}

// Propose submits a key-value update to the Raft cluster for consensus.
func (s *Store) Propose(key, val string) {
	body, _ := json.Marshal(kv{Key: key, Val: val})
	s.proposeC <- string(body)
}
