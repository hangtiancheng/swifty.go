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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func (ctx *Context) JSON(obj interface{}) {
	ctx.Type = "application/json"
	ctx.Body = obj
}

func (ctx *Context) String(format string, values ...interface{}) {
	ctx.Type = "text/plain"
	ctx.Body = fmt.Sprintf(format, values...)
}

func (ctx *Context) Data(data []byte) {
	ctx.Body = data
}

func (ctx *Context) HTML(name string, data interface{}) {
	ctx.Type = "text/html"
	ctx.Body = htmlPayload{name: name, data: data}
}

func (ctx *Context) Redirect(url string) {
	ctx.Status = http.StatusFound
	ctx.statusSet = true
	ctx.headers["Location"] = url
}

type htmlPayload struct {
	name string
	data interface{}
}

func (ctx *Context) respond() {
	if ctx.flushed {
		return
	}

	if ctx.Body != nil && !ctx.statusSet && ctx.Status == http.StatusNotFound {
		ctx.Status = http.StatusOK
	}

	for k, v := range ctx.headers {
		ctx.Writer.Header().Set(k, v)
	}

	if ctx.Body == nil {
		ctx.Writer.WriteHeader(ctx.Status)
		return
	}

	switch body := ctx.Body.(type) {
	case htmlPayload:
		ctx.respondHTML(body)
	case []byte:
		ctx.respondBytes(body)
	case string:
		ctx.respondString(body)
	case io.Reader:
		ctx.respondReader(body)
	default:
		ctx.respondJSON(body)
	}
}

func (ctx *Context) respondJSON(obj interface{}) {
	if ctx.Type == "" {
		ctx.Type = "application/json"
	}
	ctx.Writer.Header().Set("Content-Type", ctx.Type)
	ctx.Writer.WriteHeader(ctx.Status)
	data, err := json.Marshal(obj)
	if err != nil {
		return
	}
	_, _ = ctx.Writer.Write(data)
}

func (ctx *Context) respondString(s string) {
	if ctx.Type == "" {
		ctx.Type = "text/plain"
	}
	ctx.Writer.Header().Set("Content-Type", ctx.Type)
	ctx.Writer.WriteHeader(ctx.Status)
	_, _ = ctx.Writer.Write([]byte(s))
}

func (ctx *Context) respondBytes(data []byte) {
	if ctx.Type != "" {
		ctx.Writer.Header().Set("Content-Type", ctx.Type)
	}
	ctx.Writer.WriteHeader(ctx.Status)
	_, _ = ctx.Writer.Write(data)
}

func (ctx *Context) respondReader(r io.Reader) {
	if ctx.Type != "" {
		ctx.Writer.Header().Set("Content-Type", ctx.Type)
	}
	ctx.Writer.WriteHeader(ctx.Status)
	_, _ = io.Copy(ctx.Writer, r)
}

func (ctx *Context) respondHTML(payload htmlPayload) {
	if ctx.app == nil || ctx.app.htmlTemplates == nil {
		ctx.Writer.WriteHeader(http.StatusInternalServerError)
		_, _ = ctx.Writer.Write([]byte(`{"message":"HTML templates are not loaded"}`))
		return
	}
	ctx.Writer.Header().Set("Content-Type", "text/html")
	ctx.Writer.WriteHeader(ctx.Status)
	_ = ctx.app.htmlTemplates.ExecuteTemplate(ctx.Writer, payload.name, payload.data)
}
