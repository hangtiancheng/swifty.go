// Copyright (c) 2026 hangtiancheng
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

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
	dsn      = "enter mysql dsn"
	network  = "tcp"
	address  = "enter redis ip:port"
	password = "enter redis password"
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

	// Construct TCC components
	componentA := NewMockComponent(componentAID, redisClient)
	componentB := NewMockComponent(componentBID, redisClient)
	componentC := NewMockComponent(componentCID, redisClient)

	// Construct the transaction log storage module
	txRecordDAO := dao.NewTXRecordDAO(mysqlDB)
	txStore := NewMockTXStore(txRecordDAO, redisClient)

	txManager := tcc_demo.NewTXManager(txStore, tcc_demo.WithMonitorTick(time.Second))
	defer txManager.Stop()

	// Register all components
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
			Request: map[string]any{
				"biz_id": componentAID + "_biz",
			},
		},
		{ComponentID: componentBID,
			Request: map[string]any{
				"biz_id": componentBID + "_biz",
			},
		},
		{ComponentID: componentCID,
			Request: map[string]any{
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
