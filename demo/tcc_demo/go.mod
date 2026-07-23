module github.com/hangtiancheng/swifty.go/demo/tcc_demo

go 1.26

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/agiledragon/gomonkey/v2 v2.11.0
	github.com/demdxx/gocast v1.2.0
	github.com/google/uuid v1.6.0
	github.com/hangtiancheng/swifty.go/demo/redis_lock v0.0.0
	github.com/spf13/cast v1.10.0
	github.com/stretchr/testify v1.11.1
	go.uber.org/zap v1.28.0
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
	gorm.io/driver/mysql v1.6.0
	gorm.io/gorm v1.31.2
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/go-sql-driver/mysql v1.10.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/klauspost/cpuid/v2 v2.4.0 // indirect
	github.com/pkg/errors v0.9.2-0.20201214064552-5dd12d0cfe7f // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/redis/go-redis/v9 v9.21.0 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/hangtiancheng/swifty.go/demo/redis_lock => ../redis_lock
