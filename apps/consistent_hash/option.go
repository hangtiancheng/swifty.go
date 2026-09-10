package consistent_hash

// ConsistentHashOptions collects the tunables of a ConsistentHash instance.
type ConsistentHashOptions struct {
	lockExpireSeconds int
	replicas          int
}

// ConsistentHashOption configures a ConsistentHash instance.
type ConsistentHashOption func(opts *ConsistentHashOptions)

// WithLockExpireSeconds sets the expiry of the distributed ring lock.
func WithLockExpireSeconds(seconds int) ConsistentHashOption {
	return func(opts *ConsistentHashOptions) {
		opts.lockExpireSeconds = seconds
	}
}

// WithReplicas sets how many virtual nodes are created per unit of node
// weight. Every virtual node is an independent point on the hash ring.
func WithReplicas(replicas int) ConsistentHashOption {
	return func(opts *ConsistentHashOptions) {
		opts.replicas = replicas
	}
}

// repair fills unset options with sensible defaults.
func repair(opts *ConsistentHashOptions) {
	// When unset, the ring lock expires after 15 seconds.
	if opts.lockExpireSeconds <= 0 {
		opts.lockExpireSeconds = 15
	}

	if opts.replicas <= 0 {
		opts.replicas = 5
	}
}
