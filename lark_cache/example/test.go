package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/hangtiancheng/lark_cache"
)

func main() {
	port := flag.Int("port", 8001, "node port")
	nodeID := flag.String("node", "A", "node identifier")
	flag.Parse()

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("[node %s] starting at %s", *nodeID, addr)

	node, err := lark_cache.NewServer(addr, "lark_cache",
		lark_cache.WithEtcdEndpoints([]string{"localhost:2379"}),
		lark_cache.WithDialTimeout(5*time.Second),
	)
	if err != nil {
		log.Fatal("failed to create node:", err)
	}

	picker, err := lark_cache.NewClientPicker(addr)
	if err != nil {
		log.Fatal("failed to create node picker:", err)
	}

	group := lark_cache.NewGroup("test", 2<<20, lark_cache.GetterFunc(
		func(ctx context.Context, key string) ([]byte, error) {
			log.Printf("[node %s] loading from data source: key=%s", *nodeID, key)
			return fmt.Appendf(nil, "data-source-value-from-node-%s", *nodeID), nil
		}),
	)
	group.RegisterPeers(picker)

	go func() {
		log.Printf("[node %s] starting service...", *nodeID)
		if err := node.Start(); err != nil {
			log.Fatal("failed to start node:", err)
		}
	}()

	log.Printf("[node %s] waiting for registration...", *nodeID)
	time.Sleep(5 * time.Second)

	ctx := context.Background()
	localKey := fmt.Sprintf("key_%s", *nodeID)
	localValue := []byte(fmt.Sprintf("data-from-node-%s", *nodeID))

	fmt.Printf("\n=== node %s: set local data ===\n", *nodeID)
	if err := group.Set(ctx, localKey, localValue); err != nil {
		log.Fatal("failed to set local data:", err)
	}
	fmt.Printf("node %s: set key %s successfully\n", *nodeID, localKey)

	log.Printf("[node %s] waiting for other nodes...", *nodeID)
	time.Sleep(30 * time.Second)
	picker.PrintPeers()

	fmt.Printf("\n=== node %s: get local data ===\n", *nodeID)
	fmt.Printf("querying local cache directly...\n")
	stats := group.Stats()
	fmt.Printf("cache stats: %+v\n", stats)

	if val, err := group.Get(ctx, localKey); err == nil {
		fmt.Printf("node %s: local key %s = %s\n", *nodeID, localKey, val.String())
	} else {
		fmt.Printf("node %s: failed to get local key: %v\n", *nodeID, err)
	}

	otherKeys := []string{"key_A", "key_B", "key_C"}
	for _, key := range otherKeys {
		if key == localKey {
			continue
		}
		fmt.Printf("\n=== node %s: get remote data %s ===\n", *nodeID, key)
		log.Printf("[node %s] looking up remote node for key %s", *nodeID, key)
		if val, err := group.Get(ctx, key); err == nil {
			fmt.Printf("node %s: remote key %s = %s\n", *nodeID, key, val.String())
		} else {
			fmt.Printf("node %s: failed to get remote key: %v\n", *nodeID, err)
		}
	}

	select {}
}
