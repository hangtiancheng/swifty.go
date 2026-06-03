package load_balance

import "github.com/hangtiancheng/lark_rpc/internal/registry"

type LoadBalancer interface {
	Select([]registry.Instance) registry.Instance
}
