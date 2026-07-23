module github.com/hangtiancheng/swifty.go/demo/timer_demo

go 1.26

require (
	github.com/bytedance/gopkg v0.1.4
	github.com/hangtiancheng/swifty.go/swifty_http v0.0.0-00010101000000-000000000000
	github.com/prometheus/client_golang v1.23.2
	github.com/redis/go-redis/v9 v9.5.1
	go.uber.org/dig v1.19.0
	gopkg.in/yaml.v3 v3.0.1
	gorm.io/driver/mysql v1.6.0
	gorm.io/gorm v1.31.2
)

replace github.com/hangtiancheng/swifty.go/swifty_http => ../../swifty_http
