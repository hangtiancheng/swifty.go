package redis

const (
	// DefaultIdleTimeoutSeconds is the default number of seconds after which
	// idle connections are closed.
	DefaultIdleTimeoutSeconds = 10
	// DefaultMaxActive is the default maximum number of connections in the pool.
	DefaultMaxActive = 100
	// DefaultMaxIdle is the default maximum number of idle connections kept in the pool.
	DefaultMaxIdle = 20
)

// ClientOptions holds the configuration of a redis Client.
type ClientOptions struct {
	maxIdle            int
	idleTimeoutSeconds int
	maxActive          int
	wait               bool
	// Required parameters.
	network  string
	address  string
	password string
}

// ClientOption customizes a ClientOptions.
type ClientOption func(c *ClientOptions)

// WithMaxIdle sets the maximum number of idle connections kept in the pool.
func WithMaxIdle(maxIdle int) ClientOption {
	return func(c *ClientOptions) {
		c.maxIdle = maxIdle
	}
}

// WithIdleTimeoutSeconds sets the number of seconds after which idle
// connections are closed.
func WithIdleTimeoutSeconds(idleTimeoutSeconds int) ClientOption {
	return func(c *ClientOptions) {
		c.idleTimeoutSeconds = idleTimeoutSeconds
	}
}

// WithMaxActive sets the maximum number of connections in the pool (pool size).
func WithMaxActive(maxActive int) ClientOption {
	return func(c *ClientOptions) {
		c.maxActive = maxActive
	}
}

// WithWaitMode makes the client wait for a free connection when the pool is
// exhausted. go-redis always blocks on pool exhaustion (bounded by the
// caller's context), so this option is accepted for API compatibility and is
// effectively the default behavior.
func WithWaitMode() ClientOption {
	return func(c *ClientOptions) {
		c.wait = true
	}
}

// repairClient replaces zero values with the package defaults.
func repairClient(c *ClientOptions) {
	if c.maxIdle <= 0 {
		c.maxIdle = DefaultMaxIdle
	}

	if c.idleTimeoutSeconds <= 0 {
		c.idleTimeoutSeconds = DefaultIdleTimeoutSeconds
	}

	if c.maxActive <= 0 {
		c.maxActive = DefaultMaxActive
	}
}
