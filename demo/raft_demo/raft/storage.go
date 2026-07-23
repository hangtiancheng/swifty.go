package raft

import (
	"errors"
	"sync"
)

var ErrCompacted = errors.New("requested index is unavailable due to compaction")

var ErrUnavailable = errors.New("request entry at index is unavailable")

type Storage interface {
	InitialState() (HardState, ConfState, error)
	// Entries returns log entries in the range [l, r)
	Entries(l, r uint64) ([]Entry, error)
	// Term returns the term of the entry at the given index
	Term(i uint64) (uint64, error)
	// LastIndex returns the index of the last persisted entry
	LastIndex() (uint64, error)
	// FirstIndex returns the index of the first persisted entry
	FirstIndex() (uint64, error)
}

type MemoryStorage struct {
	sync.Mutex
	hardState HardState
	// Persisted log entries
	entries []Entry
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		entries: make([]Entry, 1),
	}
}

func (m *MemoryStorage) InitialState() (HardState, ConfState, error) {
	return m.hardState, ConfState{}, nil
}

func (m *MemoryStorage) Entries(l, r uint64) ([]Entry, error) {
	m.Lock()
	defer m.Unlock()

	offset := m.entries[0].Index
	if l <= offset {
		return nil, ErrCompacted
	}

	if r > m.lastIndex()+1 {
		return nil, ErrUnavailable
	}

	if len(m.entries) == 1 {
		return nil, ErrUnavailable
	}

	return m.entries[l-offset : r-offset], nil

}

func (m *MemoryStorage) Term(i uint64) (uint64, error) {
	m.Lock()
	defer m.Unlock()
	offset := m.entries[0].Index
	if i < offset {
		return 0, ErrCompacted
	}

	if int(i-offset) >= len(m.entries) {
		return 0, ErrUnavailable
	}

	return m.entries[i-offset].Term, nil
}

func (m *MemoryStorage) LastIndex() (uint64, error) {
	m.Lock()
	defer m.Unlock()
	return m.lastIndex(), nil
}

func (m *MemoryStorage) lastIndex() uint64 {
	return m.entries[0].Index + uint64(len(m.entries)) - 1
}

func (m *MemoryStorage) FirstIndex() (uint64, error) {
	m.Lock()
	defer m.Lock()
	return m.firstIndex(), nil
}

func (m *MemoryStorage) firstIndex() uint64 {
	return m.entries[0].Index + 1
}
