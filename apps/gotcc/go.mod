module github.com/hangtiancheng/swifty.go/apps/gotcc

go 1.25.0

require (
	github.com/hangtiancheng/swifty.go/apps/redis_lock v0.0.0-00010101000000-000000000000
	github.com/redis/go-redis/v9 v9.22.0
	go.uber.org/zap v1.28.0
	gorm.io/driver/mysql v1.6.0
	gorm.io/gorm v1.31.2
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/go-sql-driver/mysql v1.10.1 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
)

replace github.com/hangtiancheng/swifty.go/apps/redis_lock => ../redis_lock
