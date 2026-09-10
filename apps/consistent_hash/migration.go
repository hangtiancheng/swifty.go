package consistent_hash

import (
	"context"
	"errors"
	"math"
)

// Migrator is the user supplied callback that actually moves data. For every
// planned migration it receives the data keys plus the source and destination
// node ids.
type Migrator func(ctx context.Context, dataKeys map[string]struct{}, from, to string) error

// migrateIn plans the migration triggered by adding a virtual node with the
// given score: data keys that used to be owned by the successor node but now
// fall into the new node's range are handed over to nodeID.
func (c *ConsistentHash) migrateIn(ctx context.Context, virtualScore int32, nodeID string) (from, to string, datas map[string]struct{}, err error) {
	// No migrator registered, nothing to plan.
	if c.migrator == nil {
		return
	}

	// Look up the nodes registered under the new score. Several virtual nodes
	// may share one score; in that case the newcomer owns no data yet.
	nodes, err := c.hashRing.Node(ctx, virtualScore)
	if err != nil {
		return
	}

	if len(nodes) > 1 {
		return
	}

	// Find the predecessor score. When it is missing or equals the new score,
	// the new virtual node is the only one on the ring, so nothing moves.
	lastScore, err := c.hashRing.Floor(ctx, c.decrScore(virtualScore))
	if err != nil {
		return
	}

	if lastScore == -1 || lastScore == virtualScore {
		return
	}

	// Find the successor score; the same reasoning applies.
	nextScore, err := c.hashRing.Ceiling(ctx, c.incrScore(virtualScore))
	if err != nil {
		return
	}

	if nextScore == -1 || nextScore == virtualScore {
		return
	}

	// patternOne: last-0-cur, the predecessor wraps around the ring end.
	patternOne := lastScore > virtualScore
	// patternTwo: cur-0-next, the successor wraps around the ring end.
	patternTwo := nextScore < virtualScore
	if patternOne {
		lastScore -= math.MaxInt32
	}

	if patternTwo {
		virtualScore -= math.MaxInt32
		lastScore -= math.MaxInt32
	}

	// The data keys that fall into the new range used to belong to the owner
	// of the successor score.
	nextNodes, err := c.hashRing.Node(ctx, nextScore)
	if err != nil {
		return
	}

	if len(nextNodes) == 0 {
		return
	}

	var dataKeys map[string]struct{}
	if dataKeys, err = c.hashRing.DataKeys(ctx, c.getNodeID(nextNodes[0])); err != nil {
		return
	}

	datas = make(map[string]struct{})
	for dataKey := range dataKeys {
		dataScore := c.encryptor.Encrypt(dataKey)
		if patternOne && dataScore > (lastScore+math.MaxInt32) {
			dataScore -= math.MaxInt32
		}

		if patternTwo {
			dataScore -= math.MaxInt32
		}

		if dataScore <= lastScore || dataScore > virtualScore {
			continue
		}

		// This data key has to migrate.
		datas[dataKey] = struct{}{}
	}

	if err = c.hashRing.DeleteNodeToDataKeys(ctx, c.getNodeID(nextNodes[0]), datas); err != nil {
		return "", "", nil, err
	}

	if err = c.hashRing.AddNodeToDataKeys(ctx, nodeID, datas); err != nil {
		return "", "", nil, err
	}

	return c.getNodeID(nextNodes[0]), nodeID, datas, nil
}

// migrateOut plans the migration triggered by removing a virtual node with the
// given score: the data keys owned by nodeID inside the node's range are
// handed over to the successor node.
func (c *ConsistentHash) migrateOut(ctx context.Context, virtualScore int32, nodeID string) (from, to string, datas map[string]struct{}, err error) {
	// No migrator registered, nothing to plan.
	if c.migrator == nil {
		return
	}

	// Hand over the affected data keys once the destination is known.
	defer func() {
		if err != nil {
			return
		}
		if to == "" || len(datas) == 0 {
			return
		}

		if err = c.hashRing.DeleteNodeToDataKeys(ctx, nodeID, datas); err != nil {
			return
		}

		err = c.hashRing.AddNodeToDataKeys(ctx, to, datas)
	}()

	from = nodeID

	nodes, nodeErr := c.hashRing.Node(ctx, virtualScore)
	if nodeErr != nil {
		err = nodeErr
		return
	}

	if len(nodes) == 0 {
		return
	}

	// When several virtual nodes share the score, only the first registered
	// one drives the migration for its range.
	if c.getNodeID(nodes[0]) != nodeID {
		return
	}

	allDataKeys, err := c.hashRing.DataKeys(ctx, nodeID)
	if err != nil {
		return
	}

	if len(allDataKeys) == 0 {
		return
	}

	// Only the keys inside (lastScore, virtualScore] belong to this virtual
	// node.
	lastScore, err := c.hashRing.Floor(ctx, c.decrScore(virtualScore))
	if err != nil {
		return
	}

	var onlyScore bool
	if lastScore == -1 || lastScore == virtualScore {
		if len(nodes) == 1 {
			err = errors.New("no other node on the ring to migrate data to")
			return
		}
		// The removed score is the only score on the ring, so every data key
		// has to move.
		onlyScore = true
	}

	// The ring wraps around at math.MaxInt32; when the predecessor score is
	// numerically greater, linearize the segment below zero.
	wrapped := lastScore > virtualScore
	if wrapped {
		lastScore -= math.MaxInt32
	}

	datas = make(map[string]struct{})
	for dataKey := range allDataKeys {
		if onlyScore {
			datas[dataKey] = struct{}{}
			continue
		}
		dataScore := c.encryptor.Encrypt(dataKey)
		if wrapped && dataScore > lastScore+math.MaxInt32 {
			dataScore -= math.MaxInt32
		}
		if dataScore <= lastScore || dataScore > virtualScore {
			continue
		}
		datas[dataKey] = struct{}{}
	}

	// When several virtual nodes share this score, delegate the data to the
	// next registered one.
	if len(nodes) > 1 {
		to = c.getNodeID(nodes[1])
		return
	}

	// Otherwise hand the data to the successor node on the ring.
	to, err = c.getValidNextNode(ctx, virtualScore, nodeID, nil)
	if err != nil {
		return
	}

	if to == "" {
		err = errors.New("no other node on the ring to migrate data to")
	}

	return
}

// getValidNextNode searches for the nearest successor node whose id differs
// from nodeID, skipping scores that were already inspected.
func (c *ConsistentHash) getValidNextNode(ctx context.Context, score int32, nodeID string, ranged map[int32]struct{}) (string, error) {
	nextScore, err := c.hashRing.Ceiling(ctx, c.incrScore(score))
	if err != nil {
		return "", err
	}
	if nextScore == -1 {
		return "", nil
	}

	if _, ok := ranged[nextScore]; ok {
		return "", nil
	}

	nextNodes, err := c.hashRing.Node(ctx, nextScore)
	if err != nil {
		return "", err
	}

	if len(nextNodes) == 0 {
		return "", errors.New("next node empty")
	}

	if nextNode := c.getNodeID(nextNodes[0]); nextNode != nodeID {
		return nextNode, nil
	}

	if len(nextNodes) > 1 {
		return c.getNodeID(nextNodes[1]), nil
	}

	if ranged == nil {
		ranged = make(map[int32]struct{})
	}
	ranged[score] = struct{}{}

	return c.getValidNextNode(ctx, nextScore, nodeID, ranged)
}

// incrScore returns the successor position on the ring, wrapping around at the
// ring end.
func (c *ConsistentHash) incrScore(score int32) int32 {
	if score == math.MaxInt32-1 {
		return 0
	}
	return score + 1
}

// decrScore returns the predecessor position on the ring, wrapping around at
// the ring start.
func (c *ConsistentHash) decrScore(score int32) int32 {
	if score == 0 {
		return math.MaxInt32 - 1
	}
	return score - 1
}
