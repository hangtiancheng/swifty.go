package gotcc

import "context"

// TCCReq is the request payload delivered to a TCC component.
type TCCReq struct {
	// ComponentID is the unique component identifier.
	ComponentID string `json:"componentID"`
	// TXID is the globally unique transaction id.
	TXID string `json:"txID"`
	// Data carries the component specific business input.
	Data map[string]any `json:"data"`
}

// TCCResp is the response returned by a TCC component.
type TCCResp struct {
	ComponentID string `json:"componentID"`
	// ACK reports whether the requested phase succeeded.
	ACK  bool   `json:"ack"`
	TXID string `json:"txID"`
}

// TCCComponent is a single participant of a TCC distributed transaction.
type TCCComponent interface {
	// ID returns the unique identifier of the component.
	ID() string
	// Try executes the first phase of the two-phase commit.
	Try(ctx context.Context, req *TCCReq) (*TCCResp, error)
	// Confirm executes the second-phase confirm operation.
	Confirm(ctx context.Context, txID string) (*TCCResp, error)
	// Cancel executes the second-phase cancel operation.
	Cancel(ctx context.Context, txID string) (*TCCResp, error)
}
