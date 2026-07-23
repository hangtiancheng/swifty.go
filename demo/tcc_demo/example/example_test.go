package example

import (
	"context"
	"fmt"
	"time"

	"github.com/hangtiancheng/swifty.go/demo/tcc_demo"
	"github.com/hangtiancheng/swifty.go/demo/tcc_demo/example/dao"
	"github.com/hangtiancheng/swifty.go/demo/tcc_demo/example/pkg"
)

const (
	dsn      = "请输入 mysql sdn"
	network  = "tcp"
	address  = "请输入 redis ip:port"
	password = "请输入 redis 密码"
)

func main() {
	redisClient := pkg.NewRedisClient(network, address, password)
	mysqlDB, err := pkg.NewDB(dsn)
	if err != nil {
		fmt.Println(err)
		return
	}

	componentAID := "componentA"
	componentBID := "componentB"
	componentCID := "componentC"

	// 构造出对应的 tcc component
	componentA := NewMockComponent(componentAID, redisClient)
	componentB := NewMockComponent(componentBID, redisClient)
	componentC := NewMockComponent(componentCID, redisClient)

	// 构造出事务日志存储模块
	txRecordDAO := dao.NewTXRecordDAO(mysqlDB)
	txStore := NewMockTXStore(txRecordDAO, redisClient)

	txManager := tcc_demo.NewTXManager(txStore, tcc_demo.WithMonitorTick(time.Second))
	defer txManager.Stop()

	// 完成各组件的注册
	if err := txManager.Register(componentA); err != nil {
		fmt.Println(err)
		return
	}

	if err := txManager.Register(componentB); err != nil {
		fmt.Println(err)
		return
	}

	if err := txManager.Register(componentC); err != nil {
		fmt.Println(err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	_, success, err := txManager.Transaction(ctx, []*tcc_demo.RequestEntity{
		{ComponentID: componentAID,
			Request: map[string]interface{}{
				"biz_id": componentAID + "_biz",
			},
		},
		{ComponentID: componentBID,
			Request: map[string]interface{}{
				"biz_id": componentBID + "_biz",
			},
		},
		{ComponentID: componentCID,
			Request: map[string]interface{}{
				"biz_id": componentCID + "_biz",
			},
		},
	}...)
	if err != nil {
		fmt.Printf("tx failed, err: %v", err)
		return
	}
	if !success {
		fmt.Println("tx failed")
		return
	}

	<-time.After(2 * time.Second)

	fmt.Println("success")
}
