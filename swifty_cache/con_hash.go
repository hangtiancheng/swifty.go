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

package swifty_cache

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// ConHashMap is a concurrent consistent hash ring with virtual nodes.
type ConHashMap struct {
	mu            sync.RWMutex
	config        *ConHashConfig
	keys          []int
	hashMap       map[int]string
	nodeReplicas  map[string]int
	nodeCounts    map[string]int64
	totalRequests int64
}

// NewConHash creates a consistent hash ring.
func NewConHash(opts ...ConHashOption) *ConHashMap {
	m := &ConHashMap{
		config:       DefaultConHashConfig,
		hashMap:      make(map[int]string),
		nodeReplicas: make(map[string]int),
		nodeCounts:   make(map[string]int64),
	}

	for _, opt := range opts {
		opt(m)
	}
	m.startBalancer()
	return m
}

// Option configures a Map.
type ConHashOption func(*ConHashMap)

// WithConfig sets the hash ring config.
func WithConHashConfig(config *ConHashConfig) ConHashOption {
	return func(m *ConHashMap) {
		if config != nil {
			m.config = config
		}
	}
}

// Add adds nodes to the hash ring.
func (m *ConHashMap) Add(nodes ...string) error {
	if len(nodes) == 0 {
		return errors.New("no nodes provided")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, node := range nodes {
		if node == "" {
			continue
		}
		m.addNodeLocked(node, m.config.DefaultReplicas)
	}
	sort.Ints(m.keys)
	return nil
}

// Remove removes a node from the hash ring.
func (m *ConHashMap) Remove(node string) error {
	if node == "" {
		return errors.New("invalid node")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.removeNodeLocked(node); err != nil {
		return err
	}
	sort.Ints(m.keys)
	return nil
}

// Get returns the node responsible for key.
func (m *ConHashMap) Get(key string) string {
	if key == "" {
		return ""
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.keys) == 0 {
		return ""
	}

	hash := int(m.config.HashFunc([]byte(key)))
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})
	if idx == len(m.keys) {
		idx = 0
	}

	node := m.hashMap[m.keys[idx]]
	m.nodeCounts[node]++
	atomic.AddInt64(&m.totalRequests, 1)
	return node
}

func (m *ConHashMap) addNodeLocked(node string, replicas int) {
	for i := 0; i < replicas; i++ {
		hash := int(m.config.HashFunc([]byte(fmt.Sprintf("%s-%d", node, i))))
		m.keys = append(m.keys, hash)
		m.hashMap[hash] = node
	}
	m.nodeReplicas[node] = replicas
	if _, ok := m.nodeCounts[node]; !ok {
		m.nodeCounts[node] = 0
	}
}

func (m *ConHashMap) removeNodeLocked(node string) error {
	replicas := m.nodeReplicas[node]
	if replicas == 0 {
		return fmt.Errorf("node %s not found", node)
	}

	for i := 0; i < replicas; i++ {
		hash := int(m.config.HashFunc([]byte(fmt.Sprintf("%s-%d", node, i))))
		delete(m.hashMap, hash)
		for j := 0; j < len(m.keys); j++ {
			if m.keys[j] == hash {
				m.keys = append(m.keys[:j], m.keys[j+1:]...)
				break
			}
		}
	}

	delete(m.nodeReplicas, node)
	delete(m.nodeCounts, node)
	return nil
}

func (m *ConHashMap) checkAndRebalance() {
	if atomic.LoadInt64(&m.totalRequests) < 1000 {
		return
	}

	m.mu.RLock()
	if len(m.nodeReplicas) == 0 {
		m.mu.RUnlock()
		return
	}
	avgLoad := float64(atomic.LoadInt64(&m.totalRequests)) / float64(len(m.nodeReplicas))
	var maxDiff float64
	for _, count := range m.nodeCounts {
		diff := math.Abs(float64(count) - avgLoad)
		if avgLoad > 0 && diff/avgLoad > maxDiff {
			maxDiff = diff / avgLoad
		}
	}
	m.mu.RUnlock()

	if maxDiff > m.config.LoadBalanceThreshold {
		m.rebalanceNodes()
	}
}

func (m *ConHashMap) rebalanceNodes() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.nodeReplicas) == 0 {
		return
	}

	avgLoad := float64(atomic.LoadInt64(&m.totalRequests)) / float64(len(m.nodeReplicas))
	updates := make(map[string]int)
	for node, count := range m.nodeCounts {
		currentReplicas := m.nodeReplicas[node]
		loadRatio := float64(count) / avgLoad

		var newReplicas int
		if loadRatio > 1 {
			newReplicas = int(float64(currentReplicas) / loadRatio)
		} else {
			newReplicas = int(float64(currentReplicas) * (2 - loadRatio))
		}

		if newReplicas < m.config.MinReplicas {
			newReplicas = m.config.MinReplicas
		}
		if newReplicas > m.config.MaxReplicas {
			newReplicas = m.config.MaxReplicas
		}
		if newReplicas != currentReplicas {
			updates[node] = newReplicas
		}
	}

	for node, replicas := range updates {
		_ = m.removeNodeLocked(node)
		m.addNodeLocked(node, replicas)
	}
	for node := range m.nodeCounts {
		m.nodeCounts[node] = 0
	}
	atomic.StoreInt64(&m.totalRequests, 0)
	sort.Ints(m.keys)
}

// GetStats returns the traffic share per node.
func (m *ConHashMap) GetStats() map[string]float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]float64)
	total := atomic.LoadInt64(&m.totalRequests)
	if total == 0 {
		return stats
	}
	for node, count := range m.nodeCounts {
		stats[node] = float64(count) / float64(total)
	}
	return stats
}

func (m *ConHashMap) startBalancer() {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			m.checkAndRebalance()
		}
	}()
}
