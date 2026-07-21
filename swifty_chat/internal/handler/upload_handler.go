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

package handler

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/hangtiancheng/swifty.go/swifty_chat/internal/config"

	"github.com/hangtiancheng/swifty.go/swifty_http"
)

func UploadAvatar(ctx *swifty_http.Context, next func()) {
	file, header, err := ctx.FormFile("file")
	if err != nil {
		JsonBack(ctx, "file is required", -2, nil)
		return
	}
	defer file.Close()

	conf := config.Get()
	_ = os.MkdirAll(conf.Static.AvatarPath, 0755)

	filename := fmt.Sprintf("%d_%s", time.Now().UnixMilli(), header.Filename)
	dst := filepath.Join(conf.Static.AvatarPath, filename)

	out, err := os.Create(dst)
	if err != nil {
		JsonBack(ctx, "failed to save file", -1, nil)
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		JsonBack(ctx, "failed to save file", -1, nil)
		return
	}

	url := "/static/avatars/" + filename
	JsonBack(ctx, "upload successful", 0, swifty_http.H{"url": url})
}

func UploadFile(ctx *swifty_http.Context, next func()) {
	file, header, err := ctx.FormFile("file")
	if err != nil {
		JsonBack(ctx, "file is required", -2, nil)
		return
	}
	defer file.Close()

	conf := config.Get()
	_ = os.MkdirAll(conf.Static.FilePath, 0755)

	filename := fmt.Sprintf("%d_%s", time.Now().UnixMilli(), header.Filename)
	dst := filepath.Join(conf.Static.FilePath, filename)

	out, err := os.Create(dst)
	if err != nil {
		JsonBack(ctx, "failed to save file", -1, nil)
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		JsonBack(ctx, "failed to save file", -1, nil)
		return
	}

	url := "/static/files/" + filename
	JsonBack(ctx, "upload successful", 0, swifty_http.H{
		"url":       url,
		"file_name": header.Filename,
		"file_size": fmt.Sprintf("%d", header.Size),
	})
}
