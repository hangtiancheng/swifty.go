package config

// RedisConfig is the cache configuration.
type RedisConfig struct {
	Network            string `json:"network"`
	Address            string `json:"address"`
	Password           string `json:"password"`
	MaxIdle            int    `json:"maxIdle"`
	IdleTimeoutSeconds int    `json:"idleTimeout"`
	// Maximum number of connections kept alive in the pool.
	MaxActive int `json:"maxActive"`
	// When the connection limit is reached, whether new requests wait or fail immediately.
	Wait bool `json:"wait"`
}

type RedisConfigProvider struct {
	conf *RedisConfig
}

func NewRedisConfigProvider(conf *RedisConfig) *RedisConfigProvider {
	return &RedisConfigProvider{
		conf: conf,
	}
}

func (r *RedisConfigProvider) Get() *RedisConfig {
	return r.conf
}

var defaultRedisConfProvider *RedisConfigProvider

func DefaultRedisConfigProvider() *RedisConfigProvider {
	return defaultRedisConfProvider
}
