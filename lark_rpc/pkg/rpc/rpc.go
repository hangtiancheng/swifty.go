package rpc

import (
	"github.com/hangtiancheng/lark-go/lark_rpc/internal/codec"
	"github.com/hangtiancheng/lark-go/lark_rpc/internal/load_balance"
	"github.com/hangtiancheng/lark-go/lark_rpc/internal/registry"
)

type CodecType = codec.Type

type Registry = registry.Registry
type Instance = registry.Instance
type LoadBalancer = load_balance.LoadBalancer

var (
	CodecJSON  = codec.JSON
	CodecProto = codec.PROTO
)

func NewRegistry(endpoints []string) (*Registry, error) {
	return registry.NewRegistry(endpoints)
}
