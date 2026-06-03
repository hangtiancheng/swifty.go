package registry

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	client_v3 "go.etcd.io/etcd/client/v3"
)

type Config struct {
	Endpoints   []string
	DialTimeout time.Duration
}

var DefaultConfig = &Config{
	Endpoints:   []string{"localhost:2379"},
	DialTimeout: 5 * time.Second,
}

func Register(svcName, addr string, stopCh <-chan error) error {
	cli, err := client_v3.New(client_v3.Config{
		Endpoints:   DefaultConfig.Endpoints,
		DialTimeout: DefaultConfig.DialTimeout,
	})
	if err != nil {
		return fmt.Errorf("failed to create etcd client: %v", err)
	}

	localIP, err := getLocalIP()
	if err != nil {
		cli.Close()
		return fmt.Errorf("failed to get local IP: %v", err)
	}
	if addr[0] == ':' {
		addr = fmt.Sprintf("%s%s", localIP, addr)
	}

	lease, err := cli.Grant(context.Background(), 10)
	if err != nil {
		cli.Close()
		return fmt.Errorf("failed to create lease: %v", err)
	}

	key := fmt.Sprintf("/services/%s/%s", svcName, addr)
	_, err = cli.Put(context.Background(), key, addr, client_v3.WithLease(lease.ID))
	if err != nil {
		cli.Close()
		return fmt.Errorf("failed to put key-value to etcd: %v", err)
	}

	keepAliveCh, err := cli.KeepAlive(context.Background(), lease.ID)
	if err != nil {
		cli.Close()
		return fmt.Errorf("failed to keep lease alive: %v", err)
	}

	go func() {
		defer cli.Close()
		for {
			select {
			case <-stopCh:
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				cli.Revoke(ctx, lease.ID)
				cancel()
				return
			case resp, ok := <-keepAliveCh:
				if !ok {
					log.Print("keep alive channel closed")
					return
				}
				log.Printf("successfully renewed lease: %d", resp.ID)
			}
		}
	}()

	log.Printf("Service registered: %s at %s", svcName, addr)
	return nil
}

func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no valid local IP found")
}
