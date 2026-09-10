package dao

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/hangtiancheng/swifty.go/apps/gotcc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestParseComponentTryStatuses(t *testing.T) {
	statuses, err := ParseComponentTryStatuses(`{"component_a":{"componentID":"component_a","tryStatus":"hanging"}}`)
	if err != nil {
		t.Fatalf("ParseComponentTryStatuses() error = %v", err)
	}
	component, ok := statuses["component_a"]
	if !ok {
		t.Fatal("component_a missing from parsed statuses")
	}
	if component.ComponentID != "component_a" {
		t.Fatalf("component id = %q, want %q", component.ComponentID, "component_a")
	}
	if component.TryStatus != gotcc.TryHanging.String() {
		t.Fatalf("try status = %q, want %q", component.TryStatus, gotcc.TryHanging.String())
	}

	if _, err := ParseComponentTryStatuses("not-json"); err == nil {
		t.Fatal("ParseComponentTryStatuses() must fail for invalid JSON")
	}
}

func TestApplyComponentStatus(t *testing.T) {
	body := `{"component_a":{"componentID":"component_a","tryStatus":"hanging"}}`

	newBody, err := applyComponentStatus(body, "component_a", gotcc.TrySuccessful.String())
	if err != nil {
		t.Fatalf("applyComponentStatus() error = %v", err)
	}
	statuses, err := ParseComponentTryStatuses(newBody)
	if err != nil {
		t.Fatalf("ParseComponentTryStatuses() error = %v", err)
	}
	if got := statuses["component_a"].TryStatus; got != gotcc.TrySuccessful.String() {
		t.Fatalf("try status = %q, want %q", got, gotcc.TrySuccessful.String())
	}

	// Re-applying the current status is a no-op.
	sameBody, err := applyComponentStatus(newBody, "component_a", gotcc.TrySuccessful.String())
	if err != nil {
		t.Fatalf("applyComponentStatus() error = %v", err)
	}
	if sameBody != newBody {
		t.Fatalf("idempotent apply changed the body: %q -> %q", newBody, sameBody)
	}

	// An unknown component must be rejected.
	if _, err := applyComponentStatus(body, "component_b", gotcc.TrySuccessful.String()); err == nil {
		t.Fatal("applyComponentStatus() must fail for an unknown component")
	}

	// A component that is not hanging anymore must be rejected.
	failedBody := `{"component_a":{"componentID":"component_a","tryStatus":"failure"}}`
	if _, err := applyComponentStatus(failedBody, "component_a", gotcc.TrySuccessful.String()); err == nil {
		t.Fatal("applyComponentStatus() must fail for a non-hanging component")
	}

	// Invalid JSON must be rejected.
	if _, err := applyComponentStatus("not-json", "component_a", gotcc.TrySuccessful.String()); err == nil {
		t.Fatal("applyComponentStatus() must fail for invalid JSON")
	}
}

// TestTXRecordDAOIntegration exercises the GORM DAO against a live MySQL.
// It is skipped when no MySQL can be reached.
func TestTXRecordDAOIntegration(t *testing.T) {
	dsn := os.Getenv("GOTCC_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("GOTCC_TEST_MYSQL_DSN is not set; this test needs a live MySQL")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("live MySQL not reachable with GOTCC_TEST_MYSQL_DSN: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Skipf("cannot obtain the MySQL handle: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		t.Skipf("live MySQL not reachable with GOTCC_TEST_MYSQL_DSN: %v", err)
	}
	if err := db.AutoMigrate(&TXRecordPO{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	dao := NewTXRecordDAO(db)

	statuses := map[string]*ComponentTryStatus{
		"component_a": {ComponentID: "component_a", TryStatus: gotcc.TryHanging.String()},
		"component_b": {ComponentID: "component_b", TryStatus: gotcc.TryHanging.String()},
	}
	encoded, err := json.Marshal(statuses)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Create.
	record := &TXRecordPO{
		Status:               gotcc.TXHanging.String(),
		ComponentTryStatuses: string(encoded),
	}
	id, err := dao.CreateTXRecord(ctx, record)
	if err != nil {
		t.Fatalf("CreateTXRecord() error = %v", err)
	}
	if id == 0 {
		t.Fatal("CreateTXRecord() must return the auto-generated id")
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = dao.db.WithContext(cleanupCtx).Unscoped().Where("id = ?", id).Delete(&TXRecordPO{}).Error
	})

	// Query by id and status.
	records, err := dao.GetTXRecords(ctx, WithID(id), WithStatus(gotcc.TXHanging))
	if err != nil {
		t.Fatalf("GetTXRecords() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("GetTXRecords() returned %d records, want 1", len(records))
	}

	// Transition component_a to successful.
	if err := dao.UpdateComponentStatus(ctx, id, "component_a", gotcc.TrySuccessful.String()); err != nil {
		t.Fatalf("UpdateComponentStatus() error = %v", err)
	}
	records, err = dao.GetTXRecords(ctx, WithID(id))
	if err != nil {
		t.Fatalf("GetTXRecords() error = %v", err)
	}
	statuses, err = ParseComponentTryStatuses(records[0].ComponentTryStatuses)
	if err != nil {
		t.Fatalf("ParseComponentTryStatuses() error = %v", err)
	}
	if got := statuses["component_a"].TryStatus; got != gotcc.TrySuccessful.String() {
		t.Fatalf("component_a try status = %q, want %q", got, gotcc.TrySuccessful.String())
	}
	// component_b is still hanging.
	if got := statuses["component_b"].TryStatus; got != gotcc.TryHanging.String() {
		t.Fatalf("component_b try status = %q, want %q", got, gotcc.TryHanging.String())
	}

	// A second transition of component_a must fail (no longer hanging).
	if err := dao.UpdateComponentStatus(ctx, id, "component_a", gotcc.TryFailure.String()); err == nil {
		t.Fatal("UpdateComponentStatus() must fail for a component that is not hanging")
	}

	// Commit the final status via LockAndDo.
	err = dao.LockAndDo(ctx, id, func(ctx context.Context, updater TXRecordUpdater, record *TXRecordPO) error {
		if record.Status != gotcc.TXHanging.String() {
			return nil
		}
		record.Status = gotcc.TXSuccessful.String()
		return updater.UpdateTXRecord(ctx, record)
	})
	if err != nil {
		t.Fatalf("LockAndDo() error = %v", err)
	}

	records, err = dao.GetTXRecords(ctx, WithID(id))
	if err != nil {
		t.Fatalf("GetTXRecords() error = %v", err)
	}
	if records[0].Status != gotcc.TXSuccessful.String() {
		t.Fatalf("tx status = %q, want %q", records[0].Status, gotcc.TXSuccessful.String())
	}
}
