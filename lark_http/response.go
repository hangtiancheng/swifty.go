package lark_http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func (c *Context) JSON(obj interface{}) {
	c.Type = "application/json"
	c.Body = obj
}

func (c *Context) String(format string, values ...interface{}) {
	c.Type = "text/plain"
	c.Body = fmt.Sprintf(format, values...)
}

func (c *Context) Data(data []byte) {
	c.Body = data
}

func (c *Context) HTML(name string, data interface{}) {
	c.Type = "text/html"
	c.Body = htmlPayload{name: name, data: data}
}

func (c *Context) Redirect(url string) {
	c.Status = http.StatusFound
	c.statusSet = true
	c.headers["Location"] = url
}

type htmlPayload struct {
	name string
	data interface{}
}

func (c *Context) respond() {
	if c.flushed {
		return
	}

	if c.Body != nil && !c.statusSet && c.Status == http.StatusNotFound {
		c.Status = http.StatusOK
	}

	for k, v := range c.headers {
		c.Writer.Header().Set(k, v)
	}

	if c.Body == nil {
		c.Writer.WriteHeader(c.Status)
		return
	}

	switch body := c.Body.(type) {
	case htmlPayload:
		c.respondHTML(body)
	case []byte:
		c.respondBytes(body)
	case string:
		c.respondString(body)
	case io.Reader:
		c.respondReader(body)
	default:
		c.respondJSON(body)
	}
}

func (c *Context) respondJSON(obj interface{}) {
	if c.Type == "" {
		c.Type = "application/json"
	}
	c.Writer.Header().Set("Content-Type", c.Type)
	c.Writer.WriteHeader(c.Status)
	data, err := json.Marshal(obj)
	if err != nil {
		return
	}
	_, _ = c.Writer.Write(data)
}

func (c *Context) respondString(s string) {
	if c.Type == "" {
		c.Type = "text/plain"
	}
	c.Writer.Header().Set("Content-Type", c.Type)
	c.Writer.WriteHeader(c.Status)
	_, _ = c.Writer.Write([]byte(s))
}

func (c *Context) respondBytes(data []byte) {
	if c.Type != "" {
		c.Writer.Header().Set("Content-Type", c.Type)
	}
	c.Writer.WriteHeader(c.Status)
	_, _ = c.Writer.Write(data)
}

func (c *Context) respondReader(r io.Reader) {
	if c.Type != "" {
		c.Writer.Header().Set("Content-Type", c.Type)
	}
	c.Writer.WriteHeader(c.Status)
	_, _ = io.Copy(c.Writer, r)
}

func (c *Context) respondHTML(payload htmlPayload) {
	if c.engine == nil || c.engine.htmlTemplates == nil {
		c.Writer.WriteHeader(http.StatusInternalServerError)
		_, _ = c.Writer.Write([]byte(`{"message":"HTML templates are not loaded"}`))
		return
	}
	c.Writer.Header().Set("Content-Type", "text/html")
	c.Writer.WriteHeader(c.Status)
	_ = c.engine.htmlTemplates.ExecuteTemplate(c.Writer, payload.name, payload.data)
}
