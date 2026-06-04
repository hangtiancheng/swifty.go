package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hangtiancheng/lark_demo/internal/ai"
	"github.com/hangtiancheng/lark_demo/internal/code"
	"github.com/hangtiancheng/lark_demo/internal/config"
	"github.com/hangtiancheng/lark_demo/internal/store"
	"github.com/hangtiancheng/lark_demo/internal/test_util"
)

func newTestServices(t *testing.T) *Services {
	t.Helper()
	database := fmt.Sprintf("server_service_test_%d", time.Now().UnixNano())
	st, err := store.Open(test_util.MongoURI(), database)
	if err != nil {
		if test_util.IsMongoUnauthorized(err) {
			t.Skipf("MongoDB requires authentication; set MONGO_URI with credentials to run integration tests: %v", err)
		}
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := st.DropDatabase(); err != nil {
			t.Fatalf("DropDatabase returned error: %v", err)
		}
		st.Close()
	})
	cfg := config.Config{JWTKey: "secret", JWTIssuer: "issuer", JWTSubject: "subject", JWTExpire: time.Hour, AIBaseURL: "http://127.0.0.1:1", AIModelName: "test"}
	return New(cfg, st, ai.NewManager(cfg, st))
}

func TestRegisterAndLogin(t *testing.T) {
	svc := newTestServices(t)
	ctx := context.Background()
	token, username, result := svc.Register(ctx, "user@example.com", "pass")
	if result != code.OK || token == "" || username != "user@example.com" {
		t.Fatalf("Register = %q, %q, %d", token, username, result)
	}
	if _, _, result := svc.Register(ctx, "user@example.com", "pass"); result != code.UserExist {
		t.Fatalf("duplicate register code = %d", result)
	}
	if token, result := svc.Login(ctx, "user@example.com", "pass"); result != code.OK || token == "" {
		t.Fatalf("Login = %q, %d", token, result)
	}
	if _, result := svc.Login(ctx, "user@example.com", "bad"); result != code.PasswordError {
		t.Fatalf("bad password code = %d", result)
	}
	if _, result := svc.Login(ctx, "missing", "pass"); result != code.UserNotExist {
		t.Fatalf("missing user code = %d", result)
	}
}

func TestSessionAndHistory(t *testing.T) {
	svc := newTestServices(t)
	ctx := context.Background()
	sessionID, result := svc.CreateSession(ctx, "user", "hello")
	if result != code.OK || sessionID == "" {
		t.Fatalf("CreateSession = %q, %d", sessionID, result)
	}
	if sessions := svc.Sessions(ctx, "user"); len(sessions) != 1 || sessions[0].ID != sessionID {
		t.Fatalf("sessions = %+v", sessions)
	}
	if err := svc.Store.CreateMessage(ctx, sessionID, "user", "hello", true); err != nil {
		t.Fatalf("CreateMessage returned error: %v", err)
	}
	history, result := svc.History(ctx, "user", sessionID)
	if result != code.OK || len(history) != 1 || !history[0].IsUser {
		t.Fatalf("history = %+v, %d", history, result)
	}
}

func TestAnswerRejectsUnsupportedModelType(t *testing.T) {
	svc := newTestServices(t)
	answer, result := svc.Answer(context.Background(), "user", "session", "hello", "unknown")
	if result != code.ModelNotFound || answer != "" {
		t.Fatalf("Answer = %q, %d", answer, result)
	}
}

func TestAnswerAcceptsRAGModelType(t *testing.T) {
	svc := newTestServices(t)
	_, result := svc.Answer(context.Background(), "user", "session", "hello", ai.ModelOllamaRAG)
	if result == code.ModelNotFound {
		t.Fatalf("RAG model type should be accepted, got ModelNotFound")
	}
}
