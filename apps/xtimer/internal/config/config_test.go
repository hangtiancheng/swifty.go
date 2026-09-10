package config

import "testing"

func TestMergeOverridesOnlyProvidedFields(t *testing.T) {
	scheduler := *gConf.Scheduler
	defer func() { *gConf.Scheduler = scheduler }()

	if err := merge([]byte(`{"scheduler":{"bucketsNum":3}}`)); err != nil {
		t.Fatalf("merge: %v", err)
	}

	if gConf.Scheduler.BucketsNum != 3 {
		t.Fatalf("BucketsNum = %d, want 3", gConf.Scheduler.BucketsNum)
	}
	// Fields absent from the provided section keep their defaults.
	if gConf.Scheduler.WorkersNum != 100 {
		t.Fatalf("WorkersNum = %d, want the default 100", gConf.Scheduler.WorkersNum)
	}
	if gConf.Scheduler.TryLockSeconds != 70 {
		t.Fatalf("TryLockSeconds = %d, want the default 70", gConf.Scheduler.TryLockSeconds)
	}
	// Sections absent from the file are untouched.
	if gConf.Redis.Network != "tcp" {
		t.Fatalf("Redis.Network = %q, want the default %q", gConf.Redis.Network, "tcp")
	}
}

func TestMergeSkipsNullAndUnknownSections(t *testing.T) {
	mysql := *gConf.Mysql
	defer func() { *gConf.Mysql = mysql }()

	if err := merge([]byte(`{"mysql":null,"unknown":{"a":1}}`)); err != nil {
		t.Fatalf("merge: %v", err)
	}

	if gConf.Mysql.MaxOpenConns != 100 {
		t.Fatalf("MaxOpenConns = %d, want the default 100", gConf.Mysql.MaxOpenConns)
	}
}

func TestValidateAcceptsDefaultsAndRejectsBrokenConfig(t *testing.T) {
	validate() // the built-in defaults must be valid

	buckets := gConf.Scheduler.BucketsNum
	gConf.Scheduler.BucketsNum = 0
	defer func() { gConf.Scheduler.BucketsNum = buckets }()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected a panic for a non-positive bucketsNum")
		}
	}()
	validate()
}
