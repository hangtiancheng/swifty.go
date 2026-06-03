package lark_cache

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/hangtiancheng/lark-go/lark_cache/consistent_hash"
	"github.com/hangtiancheng/lark-go/lark_cache/registry"
	client_v3 "go.etcd.io/etcd/client/v3"
)

const defaultSvcName = "lark_cache"

type PeerPicker interface {
	PickPeer(key string) (peer Peer, ok bool, self bool)
	Close() error
}

type Peer interface {
	Get(group string, key string) ([]byte, error)
	Set(ctx context.Context, group string, key string, value []byte) error
	Delete(group string, key string) (bool, error)
	Close() error
}

type ClientPicker struct {
	selfAddr string
	svcName  string
	mu       sync.RWMutex
	consHash *consistent_hash.Map
	clients  map[string]*Client
	etcdCli  *client_v3.Client
	ctx      context.Context
	cancel   context.CancelFunc
}

type PickerOption func(*ClientPicker)

func WithServiceName(name string) PickerOption {
	return func(p *ClientPicker) {
		p.svcName = name
	}
}

func (p *ClientPicker) PrintPeers() {
	p.mu.RLock()
	defer p.mu.RUnlock()

	log.Printf("Discovered peers:")
	for addr := range p.clients {
		log.Printf("- %s", addr)
	}
}

func NewClientPicker(addr string, opts ...PickerOption) (*ClientPicker, error) {
	ctx, cancel := context.WithCancel(context.Background())
	picker := &ClientPicker{
		selfAddr: addr,
		svcName:  defaultSvcName,
		clients:  make(map[string]*Client),
		consHash: consistent_hash.New(),
		ctx:      ctx,
		cancel:   cancel,
	}

	for _, opt := range opts {
		opt(picker)
	}

	cli, err := client_v3.New(client_v3.Config{
		Endpoints:   registry.DefaultConfig.Endpoints,
		DialTimeout: registry.DefaultConfig.DialTimeout,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create etcd client: %v", err)
	}
	picker.etcdCli = cli

	if err := picker.startServiceDiscovery(); err != nil {
		cancel()
		cli.Close()
		return nil, err
	}

	return picker, nil
}

func (p *ClientPicker) startServiceDiscovery() error {
	if err := p.fetchAllServices(); err != nil {
		return err
	}

	go p.watchServiceChanges()
	return nil
}

func (p *ClientPicker) watchServiceChanges() {
	watcher := client_v3.NewWatcher(p.etcdCli)
	watchChan := watcher.Watch(p.ctx, "/services/"+p.svcName, client_v3.WithPrefix())

	for {
		select {
		case <-p.ctx.Done():
			watcher.Close()
			return
		case resp := <-watchChan:
			p.handleWatchEvents(resp.Events)
		}
	}
}

func (p *ClientPicker) handleWatchEvents(events []*client_v3.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, event := range events {
		addr := string(event.Kv.Value)
		if addr == p.selfAddr {
			continue
		}

		switch event.Type {
		case client_v3.EventTypePut:
			if _, exists := p.clients[addr]; !exists {
				p.set(addr)
				log.Printf("New service discovered at %s", addr)
			}
		case client_v3.EventTypeDelete:
			if client, exists := p.clients[addr]; exists {
				client.Close()
				p.remove(addr)
				log.Printf("Service removed at %s", addr)
			}
		}
	}
}

func (p *ClientPicker) fetchAllServices() error {
	ctx, cancel := context.WithTimeout(p.ctx, 3*time.Second)
	defer cancel()

	resp, err := p.etcdCli.Get(ctx, "/services/"+p.svcName, client_v3.WithPrefix())
	if err != nil {
		return fmt.Errorf("failed to get all services: %v", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, kv := range resp.Kvs {
		addr := string(kv.Value)
		if addr != "" && addr != p.selfAddr {
			p.set(addr)
			log.Printf("Discovered service at %s", addr)
		}
	}
	return nil
}

func (p *ClientPicker) set(addr string) {
	if client, err := NewClient(addr, p.svcName, p.etcdCli); err == nil {
		p.consHash.Add(addr)
		p.clients[addr] = client
		log.Printf("Successfully created client for %s", addr)
	} else {
		log.Printf("Failed to create client for %s: %v", addr, err)
	}
}

func (p *ClientPicker) remove(addr string) {
	p.consHash.Remove(addr)
	delete(p.clients, addr)
}

func (p *ClientPicker) PickPeer(key string) (Peer, bool, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if addr := p.consHash.Get(key); addr != "" {
		if client, ok := p.clients[addr]; ok {
			return client, true, addr == p.selfAddr
		}
	}
	return nil, false, false
}

func (p *ClientPicker) Close() error {
	p.cancel()
	p.mu.Lock()
	defer p.mu.Unlock()

	var errs []error
	for addr, client := range p.clients {
		if err := client.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close client %s: %v", addr, err))
		}
	}

	if err := p.etcdCli.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close etcd client: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors while closing: %v", errs)
	}
	return nil
}
