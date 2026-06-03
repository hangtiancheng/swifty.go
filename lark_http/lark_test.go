package lark_http

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNestedGroup(t *testing.T) {
	r := New()
	v1 := r.Group("/v1")
	v2 := v1.Group("/v2")
	v3 := v2.Group("/v3")
	if v2.prefix != "/v1/v2" {
		t.Fatalf("v2 prefix = %q, want /v1/v2", v2.prefix)
	}
	if v3.prefix != "/v1/v2/v3" {
		t.Fatalf("v3 prefix = %q, want /v1/v2/v3", v3.prefix)
	}
}

func TestMiddlewareOrder(t *testing.T) {
	r := New()
	var order []string
	r.Use(func(c *Context) {
		order = append(order, "root-before")
		c.Next()
		order = append(order, "root-after")
	})
	api := r.Group("/api")
	api.Use(func(c *Context) {
		order = append(order, "api-before")
		c.Next()
		order = append(order, "api-after")
	})
	api.GET("/ping", func(c *Context) {
		order = append(order, "handler")
		c.Status = http.StatusOK
		c.String("pong")
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/ping", nil))

	if rec.Code != http.StatusOK || rec.Body.String() != "pong" {
		t.Fatalf("response = %d %q", rec.Code, rec.Body.String())
	}
	want := []string{"root-before", "api-before", "handler", "api-after", "root-after"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %v, want %v", order, want)
	}
}

func TestGroupMiddlewareUsesPathBoundary(t *testing.T) {
	r := New()
	var groupMiddlewareRan bool
	r.Group("/v1").Use(func(c *Context) {
		groupMiddlewareRan = true
		c.Next()
	})
	r.GET("/v10/ping", func(c *Context) {
		c.Status = http.StatusOK
		c.String("ok")
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v10/ping", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if groupMiddlewareRan {
		t.Fatal("/v1 middleware should not run for /v10")
	}
}

func TestDefaultRecovery(t *testing.T) {
	r := Default()
	r.GET("/panic", func(c *Context) {
		panic("boom")
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestAllHTTPMethods(t *testing.T) {
	r := New()
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	for _, m := range methods {
		method := m
		switch method {
		case "GET":
			r.GET("/test", func(c *Context) { c.Status = http.StatusOK; c.String("%s", method) })
		case "POST":
			r.POST("/test", func(c *Context) { c.Status = http.StatusOK; c.String("%s", method) })
		case "PUT":
			r.PUT("/test", func(c *Context) { c.Status = http.StatusOK; c.String("%s", method) })
		case "DELETE":
			r.DELETE("/test", func(c *Context) { c.Status = http.StatusOK; c.String("%s", method) })
		case "PATCH":
			r.PATCH("/test", func(c *Context) { c.Status = http.StatusOK; c.String("%s", method) })
		case "HEAD":
			r.HEAD("/test", func(c *Context) { c.Status = http.StatusOK })
		case "OPTIONS":
			r.OPTIONS("/test", func(c *Context) { c.Status = http.StatusOK; c.String("%s", method) })
		}
	}

	for _, m := range methods {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(m, "/test", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s /test status = %d", m, rec.Code)
		}
	}
}

func TestStaticFiles(t *testing.T) {
	dir, _ := os.MkdirTemp("", "lark_http-static-*")
	defer os.RemoveAll(dir)
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello"), 0644)

	r := New()
	r.Static("/assets", dir)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/hello.txt", nil))
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "hello" {
		t.Fatalf("static hit = %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/missing.txt", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("static miss status = %d", rec.Code)
	}
}

func TestTemplates(t *testing.T) {
	dir, _ := os.MkdirTemp("", "lark_http-templates-*")
	defer os.RemoveAll(dir)
	os.WriteFile(filepath.Join(dir, "index.htmx"), []byte("Hello {{.Name | upper}}"), 0644)

	r := New()
	r.SetFuncMap(template.FuncMap{"upper": strings.ToUpper})
	r.LoadHTMLGlob(filepath.Join(dir, "*.htmx"))
	r.GET("/", func(c *Context) {
		c.Status = http.StatusCreated
		c.HTML("index.htmx", H{"Name": "lark"})
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusCreated || rec.Body.String() != "Hello LARK" {
		t.Fatalf("template response = %d %q", rec.Code, rec.Body.String())
	}
}

func TestRunReturnsListenError(t *testing.T) {
	err := New().Run("bad address")
	if err == nil {
		t.Fatal("expected listen error")
	}
}

func TestMatchGroupPath(t *testing.T) {
	if !matchGroupPath("/", "") || !matchGroupPath("/anything", "/") {
		t.Fatal("root group should match all paths")
	}
	if !matchGroupPath("/v1", "/v1") || !matchGroupPath("/v1/users", "/v1") {
		t.Fatal("group should match exact path and child paths")
	}
	if matchGroupPath("/v10", "/v1") || matchGroupPath("/api", "/v1") {
		t.Fatal("group matched an unrelated path")
	}
}
