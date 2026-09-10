package pkg

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNewDBEmptyDSN(t *testing.T) {
	if _, err := NewDB(""); err == nil {
		t.Fatal("NewDB() with an empty DSN must fail")
	}
}

func TestNewDBLive(t *testing.T) {
	dsn := os.Getenv("GOTCC_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("GOTCC_TEST_MYSQL_DSN is not set; this test needs a live MySQL")
	}

	db, err := NewDB(dsn)
	if err != nil {
		t.Skipf("live MySQL not reachable with GOTCC_TEST_MYSQL_DSN: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Skipf("cannot obtain the MySQL handle: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		t.Skipf("live MySQL not reachable with GOTCC_TEST_MYSQL_DSN: %v", err)
	}
}
