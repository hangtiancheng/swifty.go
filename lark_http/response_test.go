package lark_http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRespondJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	ctx := newContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	ctx.Status = http.StatusCreated
	ctx.JSON(H{"name": "lark"})
	ctx.respond()

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("content-type = %q", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), `"name":"lark"`) {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestRespondString(t *testing.T) {
	rec := httptest.NewRecorder()
	ctx := newContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	ctx.Status = http.StatusOK
	ctx.String("hello %s", "world")
	ctx.respond()

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "text/plain" {
		t.Fatalf("content-type = %q", rec.Header().Get("Content-Type"))
	}
	if rec.Body.String() != "hello world" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestRespondData(t *testing.T) {
	rec := httptest.NewRecorder()
	ctx := newContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	ctx.Status = http.StatusOK
	ctx.Data([]byte("raw bytes"))
	ctx.respond()

	if rec.Body.String() != "raw bytes" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestRespondNilBody(t *testing.T) {
	rec := httptest.NewRecorder()
	ctx := newContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	ctx.Status = http.StatusNoContent
	ctx.respond()

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("body should be empty, got %q", rec.Body.String())
	}
}

func TestRespondSkipsWhenFlushed(t *testing.T) {
	rec := httptest.NewRecorder()
	ctx := newContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	ctx.flushed = true
	ctx.Status = http.StatusOK
	ctx.Body = "should not appear"
	ctx.respond()

	if rec.Body.Len() != 0 {
		t.Fatalf("flushed context should not write, got %q", rec.Body.String())
	}
}

func TestRespondWithHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	ctx := newContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	ctx.Status = http.StatusOK
	ctx.Set("X-Request-Id", "abc123")
	ctx.String("ok")
	ctx.respond()

	if rec.Header().Get("X-Request-Id") != "abc123" {
		t.Fatalf("custom header missing")
	}
}

func TestAutoStatusOKWhenBodySet(t *testing.T) {
	rec := httptest.NewRecorder()
	ctx := newContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	ctx.JSON(H{"ok": true})
	ctx.respond()

	if rec.Code != http.StatusOK {
		t.Fatalf("auto status = %d, want 200", rec.Code)
	}
}

func TestAutoStatusPreservesExplicitStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	ctx := newContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	ctx.Status = http.StatusCreated
	ctx.JSON(H{"id": 1})
	ctx.respond()

	if rec.Code != http.StatusCreated {
		t.Fatalf("explicit status = %d, want 201", rec.Code)
	}
}

func TestAutoStatusPreservesSetStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	ctx := newContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	ctx.SetStatus(http.StatusAccepted)
	ctx.JSON(H{"queued": true})
	ctx.respond()

	if rec.Code != http.StatusAccepted {
		t.Fatalf("SetStatus = %d, want 202", rec.Code)
	}
}

func TestDeferredResponseCanBeModifiedByMiddleware(t *testing.T) {
	engine := New()
	engine.Use(func(c *Context) {
		c.Next()
		if c.Status == http.StatusOK {
			c.Set("X-Modified", "true")
		}
	})
	engine.GET("/test", func(c *Context) {
		c.Status = http.StatusOK
		c.JSON(H{"ok": true})
	})

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/test", nil))

	if rec.Header().Get("X-Modified") != "true" {
		t.Fatalf("middleware could not modify response after handler")
	}
}
