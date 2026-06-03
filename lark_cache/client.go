package lark_cache

import (
	"context"
	"fmt"
	"time"

	"log"

	pb "github.com/hangtiancheng/lark_cache/pb"
	client_v3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	addr    string
	svcName string
	etcdCli *client_v3.Client
	conn    *grpc.ClientConn
	grpcCli pb.LarkCacheClient
}

var _ Peer = (*Client)(nil)

func NewClient(addr string, svcName string, etcdCli *client_v3.Client) (*Client, error) {
	var err error
	if etcdCli == nil {
		etcdCli, err = client_v3.New(client_v3.Config{
			Endpoints:   []string{"localhost:2379"},
			DialTimeout: 5 * time.Second,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create etcd client: %v", err)
		}
	}

	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create grpc client: %v", err)
	}

	return &Client{
		addr:    addr,
		svcName: svcName,
		etcdCli: etcdCli,
		conn:    conn,
		grpcCli: pb.NewLarkCacheClient(conn),
	}, nil
}

func (c *Client) Get(group, key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := c.grpcCli.Get(ctx, &pb.Request{
		Group: group,
		Key:   key,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get value from lark_cache: %v", err)
	}

	return resp.GetValue(), nil
}

func (c *Client) Delete(group, key string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := c.grpcCli.Delete(ctx, &pb.Request{
		Group: group,
		Key:   key,
	})
	if err != nil {
		return false, fmt.Errorf("failed to delete value from lark_cache: %v", err)
	}

	return resp.GetValue(), nil
}

func (c *Client) Set(ctx context.Context, group, key string, value []byte) error {
	resp, err := c.grpcCli.Set(ctx, &pb.Request{
		Group: group,
		Key:   key,
		Value: value,
	})
	if err != nil {
		return fmt.Errorf("failed to set value to lark_cache: %v", err)
	}
	log.Printf("grpc set request resp: %+v", resp)

	return nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
