// Package consistent_hash provides a consistent hashing implementation with
// pluggable ring storage (an in-memory skiplist ring or a Redis sorted set
// ring), a pluggable key hashing function and automatic data migration
// planning whenever nodes join or leave the ring.
package consistent_hash

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// ConsistentHash implements consistent hashing on top of a HashRing. Whenever
// a node is added or removed, the data keys whose owner changed are planned
// into migration tasks and handed over to the user supplied Migrator.
type ConsistentHash struct {
	hashRing  HashRing
	migrator  Migrator
	encryptor Encryptor
	opts      ConsistentHashOptions
}

// NewConsistentHash builds a consistent hash instance on top of the given hash
// ring, encryptor and migrator. The migrator may be nil when data migration is
// not needed.
func NewConsistentHash(hashRing HashRing, encryptor Encryptor, migrator Migrator, opts ...ConsistentHashOption) *ConsistentHash {
	ch := ConsistentHash{
		hashRing:  hashRing,
		migrator:  migrator,
		encryptor: encryptor,
	}

	for _, opt := range opts {
		opt(&ch.opts)
	}

	repair(&ch.opts)
	return &ch
}

// AddNode adds a node to the ring and migrates the affected data keys to it.
func (c *ConsistentHash) AddNode(ctx context.Context, nodeID string, weight int) error {
	// 1. Take the global distributed lock of the ring.
	if err := c.hashRing.Lock(ctx, c.opts.lockExpireSeconds); err != nil {
		return err
	}

	defer func() {
		_ = c.hashRing.Unlock(ctx)
	}()

	// 2. Reject the request when the node already exists.
	nodes, err := c.hashRing.Nodes(ctx)
	if err != nil {
		return err
	}

	for node := range nodes {
		if node == nodeID {
			return errors.New("node already exists")
		}
	}

	// 3. Compute the number of virtual nodes from the weight and the
	// replicas option.
	replicas := c.getValidWeight(weight) * c.opts.replicas
	// 4. Store the mapping between the replica count and the node id; its
	// presence also marks the node as registered on the ring.
	if err = c.hashRing.AddNodeToReplica(ctx, nodeID, replicas); err != nil {
		return err
	}

	var migrateTasks []func()
	for i := range replicas {
		// 5. Derive the virtual node key and its score on the ring.
		nodeKey := c.getRawNodeKey(nodeID, i)
		virtualScore := c.encryptor.Encrypt(nodeKey)

		// 6. Add the virtual node to the ring.
		if err := c.hashRing.Add(ctx, virtualScore, nodeKey); err != nil {
			return err
		}

		// 7. Plan the migration triggered by this virtual node: which data
		// keys move from which node to which node.
		// from: the node the data is migrated from
		// to: the node the data is migrated to
		// data: the data keys to migrate
		from, to, datas, err := c.migrateIn(ctx, virtualScore, nodeID)
		if err != nil {
			return err
		}

		// Nothing to migrate for this virtual node.
		if len(datas) == 0 {
			continue
		}

		// Collect the migration task; tasks run in a batch before returning.
		migrateTasks = append(migrateTasks, func() {
			_ = c.migrator(ctx, datas, from, to)
		})
	}

	c.batchExecuteMigrator(migrateTasks)

	return nil
}

// RemoveNode removes a node from the ring and migrates the affected data keys
// away from it. Callers learn what has to move where through the migrator
// callback.
func (c *ConsistentHash) RemoveNode(ctx context.Context, nodeID string) error {
	// 1. Take the global distributed lock of the ring.
	if err := c.hashRing.Lock(ctx, c.opts.lockExpireSeconds); err != nil {
		return err
	}

	defer func() {
		_ = c.hashRing.Unlock(ctx)
	}()

	// 2. Fail when the node does not exist.
	nodes, err := c.hashRing.Nodes(ctx)
	if err != nil {
		return err
	}

	var (
		nodeExist bool
		replicas  int
	)
	for node, nodeReplicas := range nodes {
		if node == nodeID {
			nodeExist = true
			replicas = nodeReplicas
			break
		}
	}

	if !nodeExist {
		return errors.New("invalid node id")
	}

	if err = c.hashRing.DeleteNodeToReplica(ctx, nodeID); err != nil {
		return err
	}

	var migrateTasks []func()
	// 3. Iterate over the virtual nodes of the removed node.
	for i := 0; i < replicas; i++ {
		// 4. Derive the virtual node score.
		virtualScore := c.encryptor.Encrypt(fmt.Sprintf("%s_%d", nodeID, i))
		// 5. Remove the virtual node, planning the migration it triggers.
		from, to, datas, err := c.migrateOut(ctx, virtualScore, nodeID)
		if err != nil {
			return err
		}

		nodeKey := c.getRawNodeKey(nodeID, i)
		if err = c.hashRing.Rem(ctx, virtualScore, nodeKey); err != nil {
			return err
		}

		if len(datas) == 0 {
			continue
		}

		// Collect the migration task; tasks run in a batch before returning.
		migrateTasks = append(migrateTasks, func() {
			_ = c.migrator(ctx, datas, from, to)
		})
	}

	c.batchExecuteMigrator(migrateTasks)

	return nil
}

// batchExecuteMigrator runs all migration tasks concurrently and waits for
// them to finish. A panic inside one task neither crashes the process nor
// prevents the remaining tasks from running.
func (c *ConsistentHash) batchExecuteMigrator(migrateTasks []func()) {
	var wg sync.WaitGroup
	for _, migrateTask := range migrateTasks {
		wg.Add(1)
		go func() {
			defer func() {
				_ = recover()
				wg.Done()
			}()
			migrateTask()
		}()
	}
	wg.Wait()
}

// GetNode returns the id of the node that owns the given data key and records
// the data key under that node so that later migrations can find it.
func (c *ConsistentHash) GetNode(ctx context.Context, dataKey string) (string, error) {
	// 1. Take the global distributed lock of the ring.
	if err := c.hashRing.Lock(ctx, c.opts.lockExpireSeconds); err != nil {
		return "", err
	}

	defer func() {
		_ = c.hashRing.Unlock(ctx)
	}()

	// 2. Locate the virtual node that owns the data key.
	dataScore := c.encryptor.Encrypt(dataKey)
	ceilingScore, err := c.hashRing.Ceiling(ctx, dataScore)
	if err != nil {
		return "", err
	}

	if ceilingScore == -1 {
		return "", errors.New("no node available")
	}

	nodes, err := c.hashRing.Node(ctx, ceilingScore)
	if err != nil {
		return "", err
	}

	if len(nodes) == 0 {
		return "", errors.New("no node available with empty score")
	}

	// 3. Record the mapping between the data key and the owning node id.
	nodeID := c.getNodeID(nodes[0])
	if err = c.hashRing.AddNodeToDataKeys(ctx, nodeID, map[string]struct{}{
		dataKey: {},
	}); err != nil {
		return "", err
	}

	return nodeID, nil
}

// getValidWeight clamps the node weight into [1, 10].
func (c *ConsistentHash) getValidWeight(weight int) int {
	if weight <= 0 {
		return 1
	}

	if weight >= 10 {
		return 10
	}

	return weight
}

// getRawNodeKey builds the i-th virtual node key of the given node.
func (c *ConsistentHash) getRawNodeKey(nodeID string, index int) string {
	return fmt.Sprintf("%s_%d", nodeID, index)
}

// getNodeID strips the virtual node suffix from a raw virtual node key, e.g.
// "node_a_3" becomes "node_a".
func (c *ConsistentHash) getNodeID(rawNodeKey string) string {
	index := strings.LastIndex(rawNodeKey, "_")
	if index < 0 {
		return rawNodeKey
	}
	return rawNodeKey[:index]
}
