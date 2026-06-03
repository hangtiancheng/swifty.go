package api

import "testing"

func TestArith(t *testing.T) {
	var svc Arith
	var reply Reply
	if err := svc.Add(&Args{A: 2, B: 3}, &reply); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if reply.Result != 5 {
		t.Fatalf("Add result = %d, want 5", reply.Result)
	}
	if err := svc.Mul(&Args{A: 4, B: 5}, &reply); err != nil {
		t.Fatalf("Mul returned error: %v", err)
	}
	if reply.Result != 20 {
		t.Fatalf("Mul result = %d, want 20", reply.Result)
	}
}

func TestArith2(t *testing.T) {
	var svc Arith2
	var reply Reply
	if err := svc.Add(&Args1{A: 1, B: 2, C: 3}, &reply); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if reply.Result != 6 {
		t.Fatalf("Add result = %d, want 6", reply.Result)
	}
	if err := svc.Mul(&Args1{A: 2, B: 3, C: 4}, &reply); err != nil {
		t.Fatalf("Mul returned error: %v", err)
	}
	if reply.Result != 24 {
		t.Fatalf("Mul result = %d, want 24", reply.Result)
	}
}
