package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/hangtiancheng/swifty.go/demo/redis_demo/app"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/lib"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/lib/pool"
)

// QualityInspector is the redis_demo test driver.
type QualityInspector struct {
	times    int
	app      *app.Application
	t        *testing.T
	randInst *rand.Rand
}

func NewQualityInspector(t *testing.T, times int) *QualityInspector {
	return &QualityInspector{
		t:        t,
		times:    times,
		randInst: rand.New(rand.NewSource(lib.TimeNow().UnixNano())),
	}
}

func (q *QualityInspector) prepareApp(clean bool) error {
	// 1. Remove the AOF file to avoid stale data.
	if clean {
		_ = os.Remove("./appendonly.aof")
	}
	// 1. Create the application.
	server, err := app.ConstructServer()
	if err != nil {
		return err
	}
	q.app = app.NewApplication(server, app.SetUpConfig())
	// 2. Start the application asynchronously.
	pool.Submit(func() {
		if err := q.app.Run(); err != nil {
			q.t.Error(err)
		}
	})
	return nil
}

func (q *QualityInspector) connApp() (*net.TCPConn, error) {
	<-time.After(100 * time.Millisecond)
	// Establish a tcp connection.
	return net.DialTCP("tcp", nil, &net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 6379,
	})
}

func (q *QualityInspector) execSet(w io.Writer) {
	writer := bufio.NewWriter(w)
	for i := 0; i < 2*q.times; i++ {
		k := strconv.Itoa(i % q.times)
		v := strconv.Itoa(i % q.times)
		_, _ = writer.WriteString("*3\r\n")
		_, _ = writer.WriteString("$3\r\n")
		_, _ = writer.WriteString("set\r\n")
		_, _ = writer.WriteString(fmt.Sprintf("$%d\r\n", len(k)))
		_, _ = writer.WriteString(fmt.Sprintf("%s\r\n", k))
		_, _ = writer.WriteString(fmt.Sprintf("$%d\r\n", len(v)))
		_, _ = writer.WriteString(fmt.Sprintf("%s\r\n", v))
		if err := writer.Flush(); err != nil {
			q.t.Error(err)
		}
		q.t.Logf("set k: %s, v: %s", k, v)
	}
}

func (q *QualityInspector) readSetResp(r io.Reader) {
	reader := bufio.NewReader(r)
	for i := 0; i < q.times; i++ {
		line, _, _ := reader.ReadLine()
		q.t.Logf("set resp: %s\n", line)
	}
}

func (q *QualityInspector) execGet(w io.Writer) {
	writer := bufio.NewWriter(w)
	for i := 0; i < q.times; i++ {
		k := strconv.Itoa(i)
		_, _ = writer.WriteString("*2\r\n")
		_, _ = writer.WriteString("$3\r\n")
		_, _ = writer.WriteString("get\r\n")
		_, _ = writer.WriteString(fmt.Sprintf("$%d\r\n", len(k)))
		_, _ = writer.WriteString(fmt.Sprintf("%s\r\n", k))
		if err := writer.Flush(); err != nil {
			q.t.Error(err)
		}
		q.t.Logf("get k: %s", k)
	}
}

func (q *QualityInspector) readGetResp(r io.Reader) {
	reader := bufio.NewReader(r)
	for i := 0; i < q.times; i++ {
		_, _, _ = reader.ReadLine() // $n
		line, _, _ := reader.ReadLine()
		q.t.Logf("get resp: %s\n", line)
		expected := strconv.Itoa(i)
		if string(line) != expected {
			q.t.Errorf("get resp, expect: %s, got: %s", expected, string(line))
		}
	}
}

func Test_GoRedis_Set_Get(t *testing.T) {
	q := NewQualityInspector(t, 100)

	// 1. Start go redis. Ensure global uniqueness.
	if err := q.prepareApp(true); err != nil {
		t.Error(err)
		return
	}
	defer q.app.Stop()

	// 2. Connect to go redis.
	conn, err := q.connApp()
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.Close()

	var wg sync.WaitGroup
	// 3. Read set responses.
	wg.Add(1)
	pool.Submit(func() {
		defer wg.Done()
		q.readSetResp(conn)
	})

	// 4. Send set commands.
	wg.Add(1)
	pool.Submit(func() {
		defer wg.Done()
		q.execSet(conn)
	})

	wg.Wait()

	// 5. Read get responses.
	wg.Add(1)
	pool.Submit(func() {
		defer wg.Done()
		q.readGetResp(conn)
	})

	// 6. Send get commands.
	wg.Add(1)
	pool.Submit(func() {
		defer wg.Done()
		q.execGet(conn)
	})
	wg.Wait()

	<-time.After(time.Second)
}

func Test_GoRedis_Set(t *testing.T) {
	test_redis_demo_set(t) // 1. start redis_demo  2. set data 3. stop redis_demo
}

func test_redis_demo_set(t *testing.T) {
	q := NewQualityInspector(t, 100)

	// 1. Start go redis. Ensure global uniqueness.
	if err := q.prepareApp(true); err != nil {
		t.Error(err)
		return
	}
	defer q.app.Stop()

	// 2. Connect to go redis.
	conn, err := q.connApp()
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.Close()

	var wg sync.WaitGroup
	// 3. Read set responses.
	wg.Add(1)
	pool.Submit(func() {
		defer wg.Done()
		q.readSetResp(conn)
	})

	// 4. Send set commands.
	wg.Add(1)
	pool.Submit(func() {
		defer wg.Done()
		q.execSet(conn)
	})

	wg.Wait()
	<-time.After(2 * time.Second)
}

func Test_Aof_Get(t *testing.T) {
	test_redis_demo_aof_get(t) // 1. start redis_demo (restore via AOF) 2. get data 3. stop redis_demo
}

func test_redis_demo_aof_get(t *testing.T) {
	q := NewQualityInspector(t, 100)

	<-time.After(time.Second)
	// 1. Start go redis. Do NOT delete the AOF file.
	if err := q.prepareApp(false); err != nil {
		t.Error(err)
		return
	}
	defer q.app.Stop()

	// 2. Connect to go redis.
	<-time.After(time.Second)
	conn, err := q.connApp()
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.Close()

	var wg sync.WaitGroup
	// 5. Read get responses.
	wg.Add(1)
	pool.Submit(func() {
		defer wg.Done()
		q.readGetResp(conn)
	})

	// 6. Send get commands.
	wg.Add(1)
	pool.Submit(func() {
		defer wg.Done()
		q.execGet(conn)
	})
	wg.Wait()
}
