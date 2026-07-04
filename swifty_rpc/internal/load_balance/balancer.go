package load_balance

import "github.com/hangtiancheng/swifty.go/swifty_rpc/internal/registry"

type LoadBalancer interface {
	Select([]registry.Instance) registry.Instance
}
