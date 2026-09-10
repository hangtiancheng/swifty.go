package example_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hangtiancheng/swifty.go/apps/gotcc"
	"github.com/hangtiancheng/swifty.go/apps/gotcc/example"
	"github.com/hangtiancheng/swifty.go/apps/gotcc/example/pkg"
)

func TestMockComponentTry(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		setup     func(*fakeRedis)
		req       *gotcc.TCCReq
		expectErr bool
		ack       bool
	}{
		{
			name: "lockErr",
			setup: func(f *fakeRedis) {
				f.setNEXErr = func(string) error { return errors.New("lock err") }
			},
			req:       &gotcc.TCCReq{},
			expectErr: true,
		},
		{
			name: "getTXKeyErr",
			setup: func(f *fakeRedis) {
				f.getErr = func(key string) error {
					if key == pkg.BuildTXKey("id", "err") {
						return errors.New("get err")
					}
					return nil
				}
			},
			req: &gotcc.TCCReq{
				TXID: "err",
			},
			expectErr: true,
		},
		{
			name: "getTXKeyRepeat",
			setup: func(f *fakeRedis) {
				f.store[pkg.BuildTXKey("id", "repeat")] = example.TXConfirmed.String()
			},
			req: &gotcc.TCCReq{
				TXID: "repeat",
			},
			ack: true,
		},
		{
			name: "getTXKeyCancel",
			setup: func(f *fakeRedis) {
				f.store[pkg.BuildTXKey("id", "cancel")] = example.TXCanceled.String()
			},
			req: &gotcc.TCCReq{
				TXID: "cancel",
			},
		},
		{
			name: "setTXToBizErr",
			setup: func(f *fakeRedis) {
				f.setErr = func(key, value string) error {
					if value == "setTXToBizErr" {
						return errors.New("set tx to biz err")
					}
					return nil
				}
			},
			req: &gotcc.TCCReq{
				TXID: "tx",
				Data: map[string]any{
					"biz_id": "setTXToBizErr",
				},
			},
			expectErr: true,
		},
		{
			name: "frozeBizErr",
			setup: func(f *fakeRedis) {
				f.setNXErr = func(key, value string) error {
					if key == pkg.BuildDataKey("id", "tx", "frozeBizErr") {
						return errors.New("froze biz err")
					}
					return nil
				}
			},
			req: &gotcc.TCCReq{
				TXID: "tx",
				Data: map[string]any{
					"biz_id": "frozeBizErr",
				},
			},
			expectErr: true,
		},
		{
			name: "frozeBizFail",
			setup: func(f *fakeRedis) {
				f.setNXReply[pkg.BuildDataKey("id", "tx", "frozeBizFail")] = 0
			},
			req: &gotcc.TCCReq{
				TXID: "tx",
				Data: map[string]any{
					"biz_id": "frozeBizFail",
				},
			},
		},
		{
			name: "setTxStatusErr",
			setup: func(f *fakeRedis) {
				f.setErr = func(key, value string) error {
					if key == pkg.BuildTXKey("id", "setTxStatusErr") {
						return errors.New("set tx status err")
					}
					return nil
				}
			},
			req: &gotcc.TCCReq{
				TXID: "setTxStatusErr",
			},
			expectErr: true,
		},
		{
			name: "success",
			req: &gotcc.TCCReq{
				TXID: "success",
				Data: map[string]any{
					"biz_id": "successBiz",
				},
			},
			ack: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeRedis()
			if tt.setup != nil {
				tt.setup(fake)
			}
			mockComponent := example.NewMockComponent("id", fake)
			if mockComponent.ID() != "id" {
				t.Fatalf("ID() = %q, want %q", mockComponent.ID(), "id")
			}

			resp, err := mockComponent.Try(ctx, tt.req)
			if tt.expectErr != (err != nil) {
				t.Fatalf("Try() error = %v, wantErr %v", err, tt.expectErr)
			}
			if err != nil {
				return
			}
			if resp.ACK != tt.ack {
				t.Fatalf("Try() ack = %v, want %v", resp.ACK, tt.ack)
			}
		})
	}
}

func TestMockComponentConfirm(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		setup     func(*fakeRedis)
		txid      string
		expectErr bool
		ack       bool
	}{
		{
			name: "lockErr",
			setup: func(f *fakeRedis) {
				f.setNEXErr = func(string) error { return errors.New("lock err") }
			},
			expectErr: true,
		},
		{
			name: "getTXKeyErr",
			setup: func(f *fakeRedis) {
				f.getErr = func(key string) error {
					if key == pkg.BuildTXKey("id", "err") {
						return errors.New("get err")
					}
					return nil
				}
			},
			txid:      "err",
			expectErr: true,
		},
		{
			name: "getTXKeyRepeat",
			setup: func(f *fakeRedis) {
				f.store[pkg.BuildTXKey("id", "repeat")] = example.TXConfirmed.String()
			},
			txid: "repeat",
			ack:  true,
		},
		{
			name: "getTXKeyCancel",
			setup: func(f *fakeRedis) {
				f.store[pkg.BuildTXKey("id", "cancel")] = example.TXCanceled.String()
			},
			txid: "cancel",
		},
		{
			name: "txToBizErr",
			setup: func(f *fakeRedis) {
				f.store[pkg.BuildTXKey("id", "txToBizErr")] = example.TXTried.String()
				f.getErr = func(key string) error {
					if key == pkg.BuildTXDetailKey("id", "txToBizErr") {
						return errors.New("tx to biz err")
					}
					return nil
				}
			},
			txid:      "txToBizErr",
			expectErr: true,
		},
		{
			name: "getBizErr",
			setup: func(f *fakeRedis) {
				f.store[pkg.BuildTXKey("id", "getBizErr")] = example.TXTried.String()
				f.store[pkg.BuildTXDetailKey("id", "getBizErr")] = "tried"
				f.getErr = func(key string) error {
					if key == pkg.BuildDataKey("id", "getBizErr", "tried") {
						return errors.New("get biz err")
					}
					return nil
				}
			},
			txid:      "getBizErr",
			expectErr: true,
		},
		{
			name: "getBizUnfrozen",
			setup: func(f *fakeRedis) {
				f.store[pkg.BuildTXKey("id", "getBizUnfrozen")] = example.TXTried.String()
				f.store[pkg.BuildTXDetailKey("id", "getBizUnfrozen")] = "unfrozen"
			},
			txid: "getBizUnfrozen",
		},
		{
			name: "setBizResErr",
			setup: func(f *fakeRedis) {
				f.store[pkg.BuildTXKey("id", "setBizResErr")] = example.TXTried.String()
				f.store[pkg.BuildTXDetailKey("id", "setBizResErr")] = "tried"
				f.store[pkg.BuildDataKey("id", "setBizResErr", "tried")] = example.DataFrozen.String()
				f.setErr = func(key, value string) error {
					if key == pkg.BuildDataKey("id", "setBizResErr", "tried") {
						return errors.New("set biz res err")
					}
					return nil
				}
			},
			txid:      "setBizResErr",
			expectErr: true,
		},
		{
			name: "success",
			setup: func(f *fakeRedis) {
				f.store[pkg.BuildTXKey("id", "success")] = example.TXTried.String()
				f.store[pkg.BuildTXDetailKey("id", "success")] = "successBiz"
				f.store[pkg.BuildDataKey("id", "success", "successBiz")] = example.DataFrozen.String()
			},
			txid: "success",
			ack:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeRedis()
			if tt.setup != nil {
				tt.setup(fake)
			}
			mockComponent := example.NewMockComponent("id", fake)
			if mockComponent.ID() != "id" {
				t.Fatalf("ID() = %q, want %q", mockComponent.ID(), "id")
			}

			resp, err := mockComponent.Confirm(ctx, tt.txid)
			if tt.expectErr != (err != nil) {
				t.Fatalf("Confirm() error = %v, wantErr %v", err, tt.expectErr)
			}
			if err != nil {
				return
			}
			if resp.ACK != tt.ack {
				t.Fatalf("Confirm() ack = %v, want %v", resp.ACK, tt.ack)
			}
		})
	}
}

func TestMockComponentCancel(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		setup     func(*fakeRedis)
		txid      string
		expectErr bool
		ack       bool
	}{
		{
			name: "lockErr",
			setup: func(f *fakeRedis) {
				f.setNEXErr = func(string) error { return errors.New("lock err") }
			},
			expectErr: true,
		},
		{
			name: "getTXKeyErr",
			setup: func(f *fakeRedis) {
				f.getErr = func(key string) error {
					if key == pkg.BuildTXKey("id", "err") {
						return errors.New("get err")
					}
					return nil
				}
			},
			txid:      "err",
			expectErr: true,
		},
		{
			name: "invalidTXStatus",
			setup: func(f *fakeRedis) {
				f.store[pkg.BuildTXKey("id", "invalidTXStatus")] = example.TXConfirmed.String()
			},
			txid:      "invalidTXStatus",
			expectErr: true,
		},
		{
			name: "getBizErr",
			setup: func(f *fakeRedis) {
				f.store[pkg.BuildTXKey("id", "getBizErr")] = example.TXTried.String()
				f.getErr = func(key string) error {
					if key == pkg.BuildTXDetailKey("id", "getBizErr") {
						return errors.New("get biz err")
					}
					return nil
				}
			},
			txid:      "getBizErr",
			expectErr: true,
		},
		{
			name: "deleteBizFrozeErr",
			setup: func(f *fakeRedis) {
				f.store[pkg.BuildTXKey("id", "deleteBizFrozeErr")] = example.TXTried.String()
				f.store[pkg.BuildTXDetailKey("id", "deleteBizFrozeErr")] = "deleteBizFrozeErr"
				f.delErr = func(key string) error {
					if key == pkg.BuildDataKey("id", "deleteBizFrozeErr", "deleteBizFrozeErr") {
						return errors.New("delete biz froze err")
					}
					return nil
				}
			},
			txid:      "deleteBizFrozeErr",
			expectErr: true,
		},
		{
			name: "neverTriedIsAcked",
			txid: "neverTried",
			ack:  true,
		},
		{
			name: "success",
			setup: func(f *fakeRedis) {
				f.store[pkg.BuildTXKey("id", "success")] = example.TXTried.String()
				f.store[pkg.BuildTXDetailKey("id", "success")] = "successBiz"
				f.store[pkg.BuildDataKey("id", "success", "successBiz")] = example.DataFrozen.String()
			},
			txid: "success",
			ack:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeRedis()
			if tt.setup != nil {
				tt.setup(fake)
			}
			mockComponent := example.NewMockComponent("id", fake)
			if mockComponent.ID() != "id" {
				t.Fatalf("ID() = %q, want %q", mockComponent.ID(), "id")
			}

			resp, err := mockComponent.Cancel(ctx, tt.txid)
			if tt.expectErr != (err != nil) {
				t.Fatalf("Cancel() error = %v, wantErr %v", err, tt.expectErr)
			}
			if err != nil {
				return
			}
			if resp.ACK != tt.ack {
				t.Fatalf("Cancel() ack = %v, want %v", resp.ACK, tt.ack)
			}
			if tt.name == "neverTriedIsAcked" {
				if got := fake.store[pkg.BuildTXKey("id", "neverTried")]; got != example.TXCanceled.String() {
					t.Fatalf("tx status after cancel = %q, want %q", got, example.TXCanceled.String())
				}
			}
			if tt.name == "success" {
				if _, ok := fake.store[pkg.BuildDataKey("id", "success", "successBiz")]; ok {
					t.Fatal("frozen data record must be deleted on cancel")
				}
				if got := fake.store[pkg.BuildTXKey("id", "success")]; got != example.TXCanceled.String() {
					t.Fatalf("tx status after cancel = %q, want %q", got, example.TXCanceled.String())
				}
			}
		})
	}
}
