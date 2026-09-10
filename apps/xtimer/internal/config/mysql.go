package config

// MySQLConfig is the database configuration.
type MySQLConfig struct {
	DSN string `json:"dsn"`
	// Maximum number of open connections.
	MaxOpenConns int `json:"maxOpenConns"`
	// Maximum number of idle connections.
	MaxIdleConns int `json:"maxIdleConns"`
}

type MysqlConfProvider struct {
	conf *MySQLConfig
}

func NewMysqlConfProvider(conf *MySQLConfig) *MysqlConfProvider {
	return &MysqlConfProvider{
		conf: conf,
	}
}

func (m *MysqlConfProvider) Get() *MySQLConfig {
	return m.conf
}

var defaultMysqlConfProvider *MysqlConfProvider

func DefaultMysqlConfProvider() *MysqlConfProvider {
	return defaultMysqlConfProvider
}
