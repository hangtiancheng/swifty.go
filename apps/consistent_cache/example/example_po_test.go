package example

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestExampleWriteReadRoundTrip verifies serialization without any external
// services.
func TestExampleWriteReadRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		in   Example
	}{
		{
			name: "regular record",
			in:   Example{ID: 1, Key_: "k1", Data: "v1"},
		},
		{
			name: "empty record",
			in:   Example{},
		},
		{
			name: "unicode payload",
			in:   Example{ID: 2, Key_: "k2", Data: "hello, world"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := tt.in.Write()
			if err != nil {
				t.Fatalf("Write() error = %v", err)
			}

			var got Example
			if err := got.Read(body); err != nil {
				t.Fatalf("Read(%q) error = %v", body, err)
			}

			if !reflect.DeepEqual(tt.in, got) {
				t.Errorf("round trip mismatch: got %+v, want %+v", got, tt.in)
			}
		})
	}
}

// TestExampleMetadata checks the table/key metadata accessors.
func TestExampleMetadata(t *testing.T) {
	e := Example{Key_: "k1"}
	if got, want := e.TableName(), "example"; got != want {
		t.Errorf("TableName() = %q, want %q", got, want)
	}
	if got, want := e.KeyColumn(), "key"; got != want {
		t.Errorf("KeyColumn() = %q, want %q", got, want)
	}
	if got, want := e.Key(), "k1"; got != want {
		t.Errorf("Key() = %q, want %q", got, want)
	}
	if got := e.DataColumn(); !reflect.DeepEqual(got, []string{"data"}) {
		t.Errorf("DataColumn() = %v, want [data]", got)
	}

	// The serialized body must be valid JSON.
	body, err := e.Write()
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Errorf("serialized body is not valid JSON: %v", err)
	}
}
