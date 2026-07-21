// Copyright (c) 2026 hangtiancheng
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package swifty_http

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
	ctx.JSON(H{"name": "swifty"})
	ctx.respond()

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("content-type = %q", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), `"name":"swifty"`) {
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

func TestDeferredResponseCanBeModifiedByMiddleware(t *testing.T) {
	app := New()
	app.Use(func(ctx *Context, next func()) {
		next()
		if ctx.Status == http.StatusOK {
			ctx.Set("X-Modified", "true")
		}
	})
	app.Get("/test", func(ctx *Context, next func()) {
		ctx.Status = http.StatusOK
		ctx.JSON(H{"ok": true})
	})

	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/test", nil))

	if rec.Header().Get("X-Modified") != "true" {
		t.Fatalf("middleware could not modify response after handler")
	}
}
