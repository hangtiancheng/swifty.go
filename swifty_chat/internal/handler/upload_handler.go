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
