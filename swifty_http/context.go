package swifty_http

import (
	"encoding/json"
	"mime/multipart"
	"net/http"
)

type H map[string]interface{}

type Middleware func(ctx *Context, next func())

type Context struct {
	Request *http.Request
	Writer  http.ResponseWriter

	// request info (Koa-style fields)
	Path   string
	Method string

	// deferred response (Koa-style)
	Status int
	Body   interface{}
	Type   string

	// middleware data sharing
	State  map[string]interface{}
	Params map[string]string

	// response headers (deferred)
	headers map[string]string

	// internal
	app       *Application
	flushed   bool
	statusSet bool
}

func newContext(w http.ResponseWriter, req *http.Request) *Context {
	return &Context{
		Request: req,
		Writer:  w,
		Path:    req.URL.Path,
		Method:  req.Method,
		Status:  http.StatusNotFound,
		State:   make(map[string]interface{}),
		headers: make(map[string]string),
	}
}

func (ctx *Context) Throw(status int, msg string) {
	ctx.Status = status
	ctx.statusSet = true
	// Include "data": nil so error responses match the unified {message, data}
	// shape returned by Next.js and by the success-path ctx.JSON calls.
	ctx.Body = H{"message": msg, "data": nil}
}

func (ctx *Context) Get(header string) string {
	return ctx.Request.Header.Get(header)
}

func (ctx *Context) Set(header string, value string) {
	ctx.headers[header] = value
}

func (ctx *Context) Query(key string) string {
	return ctx.Request.URL.Query().Get(key)
}

func (ctx *Context) Param(key string) string {
	value, _ := ctx.Params[key]
	return value
}

func (ctx *Context) PostForm(key string) string {
	return ctx.Request.FormValue(key)
}

func (ctx *Context) BindJSON(out interface{}) error {
	defer ctx.Request.Body.Close()
	return json.NewDecoder(ctx.Request.Body).Decode(out)
}

func (ctx *Context) FormFile(key string) (multipart.File, *multipart.FileHeader, error) {
	return ctx.Request.FormFile(key)
}
