package lark_http

import (
	"encoding/json"
	"net/http"
)

type H map[string]interface{}

type HandlerFunc func(*Context)

type Context struct {
	Req    *http.Request
	Writer http.ResponseWriter

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
	handlers  []HandlerFunc
	index     int
	engine    *Engine
	flushed   bool
	statusSet bool
}

func newContext(w http.ResponseWriter, req *http.Request) *Context {
	return &Context{
		Req:     req,
		Writer:  w,
		Status:  http.StatusNotFound,
		State:   make(map[string]interface{}),
		headers: make(map[string]string),
		index:   -1,
	}
}

func (c *Context) Next() {
	c.index++
	for ; c.index < len(c.handlers); c.index++ {
		c.handlers[c.index](c)
	}
}

func (c *Context) Abort() {
	c.index = len(c.handlers)
}

func (c *Context) Get(header string) string {
	return c.Req.Header.Get(header)
}

func (c *Context) Set(header string, value string) {
	c.headers[header] = value
}

func (c *Context) Query(key string) string {
	return c.Req.URL.Query().Get(key)
}

func (c *Context) Param(key string) string {
	value, _ := c.Params[key]
	return value
}

func (c *Context) PostForm(key string) string {
	return c.Req.FormValue(key)
}

func (c *Context) BindJSON(out interface{}) error {
	defer c.Req.Body.Close()
	return json.NewDecoder(c.Req.Body).Decode(out)
}

func (c *Context) SetStatus(code int) {
	c.Status = code
	c.statusSet = true
}

func (c *Context) Throw(status int, msg string) {
	c.Status = status
	c.statusSet = true
	c.Body = H{"message": msg}
	c.Abort()
}

func (c *Context) Path() string {
	return c.Req.URL.Path
}

func (c *Context) Method() string {
	return c.Req.Method
}
