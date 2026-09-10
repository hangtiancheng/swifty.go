package mysql

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// testPO is a minimal Object implementation for the tests.
type testPO struct {
	ID   uint   `gorm:"primarykey"`
	Key_ string `gorm:"column:key"`
	Data string `gorm:"column:data"`
}

func (*testPO) TableName() string { return "test_pos" }
func (*testPO) KeyColumn() string { return "key" }
func (o *testPO) Key() string     { return o.Key_ }

func (o *testPO) Write() (string, error) { return o.Data, nil }
func (o *testPO) Read(body string) error { o.Data = body; return nil }

// TestPutUpsertFallbackWritesAllColumns verifies that the update fallback of
// the two-step upsert overwrites every column of the record, including
// zero-valued ones. It runs against a dry-run gorm connection, so no database
// server is needed: the create step is forced to fail with a duplicate-entry
// error and the resulting UPDATE statement is captured.
func TestPutUpsertFallbackWritesAllColumns(t *testing.T) {
	gdb, err := gorm.Open(gormmysql.New(gormmysql.Config{
		DSN:                       "root@tcp(127.0.0.1:3306)/consistent_cache",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		DryRun:                 true,
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("open dry-run gorm: %v", err)
	}

	// Force the create step of Put to take the duplicate-entry fallback and
	// capture the SQL of every update statement it produces.
	var updateSQLs []string
	gdb.Callback().Create().Replace("gorm:create", func(tx *gorm.DB) {
		tx.AddError(gorm.ErrDuplicatedKey)
	})
	if err := gdb.Callback().Update().After("gorm:update").Register("test:capture", func(tx *gorm.DB) {
		updateSQLs = append(updateSQLs, tx.Statement.SQL.String())
	}); err != nil {
		t.Fatalf("register capture callback: %v", err)
	}

	db := &DB{db: gdb}
	// Data is left zero-valued on purpose: the fallback update must still
	// overwrite the data column with the empty value.
	obj := &testPO{Key_: "k1"}
	if err := db.Put(context.Background(), obj); err != nil {
		t.Fatalf("put: %v", err)
	}

	if len(updateSQLs) != 1 {
		t.Fatalf("captured %d update statements, want 1: %v", len(updateSQLs), updateSQLs)
	}
	upd := updateSQLs[0]
	if !strings.Contains(upd, "UPDATE") {
		t.Errorf("captured statement is not an update: %s", upd)
	}
	// The zero-valued data column must be written.
	if !strings.Contains(upd, "`data`") {
		t.Errorf("fallback update does not write the zero-valued `data` column: %s", upd)
	}
	// The auto-increment primary key must not be written.
	if strings.Contains(upd, "`id`") {
		t.Errorf("fallback update must not write the primary key: %s", upd)
	}
	// The update must be scoped to the key column of the object.
	if !strings.Contains(upd, "`key` = ?") {
		t.Errorf("fallback update is not scoped to the key column: %s", upd)
	}
}

// TestIsDuplicateEntryErr verifies the duplicate-entry error detection.
func TestIsDuplicateEntryErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "plain error",
			err:  errors.New("some error"),
			want: false,
		},
		{
			name: "translated duplicate entry error",
			err:  gorm.ErrDuplicatedKey,
			want: true,
		},
		{
			name: "wrapped duplicate entry error",
			err:  fmt.Errorf("upsert failed: %w", gorm.ErrDuplicatedKey),
			want: true,
		},
		{
			name: "other gorm error",
			err:  gorm.ErrRecordNotFound,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDuplicateEntryErr(tt.err); got != tt.want {
				t.Errorf("IsDuplicateEntryErr(%v) = %t, want %t", tt.err, got, tt.want)
			}
		})
	}
}
