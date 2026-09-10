package config

type MigratorAppConf struct {
	WorkersNum                  int `json:"workersNum"`
	MigrateStepMinutes          int `json:"migrateStepMinutes"`
	MigrateSuccessExpireMinutes int `json:"migrateSuccessExpireMinutes"`
	MigrateTryLockMinutes       int `json:"migrateTryLockMinutes"`
	TimerDetailCacheMinutes     int `json:"timerDetailCacheMinutes"`
}

var defaultMigratorAppConfProvider *MigratorAppConfProvider

type MigratorAppConfProvider struct {
	conf *MigratorAppConf
}

func NewMigratorAppConfProvider(conf *MigratorAppConf) *MigratorAppConfProvider {
	return &MigratorAppConfProvider{
		conf: conf,
	}
}

func (m *MigratorAppConfProvider) Get() *MigratorAppConf {
	return m.conf
}

func DefaultMigratorAppConfProvider() *MigratorAppConfProvider {
	return defaultMigratorAppConfProvider
}
