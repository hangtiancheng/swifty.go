package app

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/hangtiancheng/lark-go/lark_http"
	rpc "github.com/hangtiancheng/lark-go/lark_rpc/pkg/rpc"
	"github.com/hangtiancheng/lark_demo/internal/ai"
	"github.com/hangtiancheng/lark_demo/internal/auth"
	"github.com/hangtiancheng/lark_demo/internal/cache"
	"github.com/hangtiancheng/lark_demo/internal/code"
	"github.com/hangtiancheng/lark_demo/internal/config"
	rpc_client "github.com/hangtiancheng/lark_demo/internal/rpc_client"
	"github.com/hangtiancheng/lark_demo/internal/service"
)

type RPCClient interface {
	Complete(ctx context.Context, req rpc_client.AIRequest) (rpc_client.AIResponse, error)
	CompleteStream(ctx context.Context, req rpc_client.AIRequest) (rpc.ClientStream, error)
}

type App struct {
	cfg      config.Config
	services *service.Services
	cache    cache.Cache
	rpc      RPCClient
}

func New(cfg config.Config, services *service.Services, cache cache.Cache, rpc RPCClient) *App {
	return &App{cfg: cfg, services: services, cache: cache, rpc: rpc}
}

func (a *App) Engine() *lark_http.Engine {
	engine := lark_http.Default()
	engine.Use(a.corsMiddleware)
	api := engine.Group("/api/v1")
	user := api.Group("/user")
	user.POST("/login", a.login)
	user.POST("/register", a.register)
	aiGroup := api.Group("/ai")
	aiGroup.Use(a.authMiddleware)
	aiGroup.GET("/chat/get-user-sessions-by-username", a.getUserSessions)
	aiGroup.POST("/chat/create-session-and-send-message", a.createSessionAndSendMessage)
	aiGroup.POST("/chat/create-session-and-send-message-stream", a.createSessionAndSendMessageStream)
	aiGroup.POST("/chat/send-message-2-session", a.sendMessageToSession)
	aiGroup.POST("/chat/send-message-stream-2-session", a.sendMessageStreamToSession)
	aiGroup.POST("/chat/get-chat-history-list", a.getChatHistoryList)
	file := api.Group("/file")
	file.Use(a.authMiddleware)
	file.POST("/upload", a.uploadFile)
	return engine
}

func (a *App) corsMiddleware(c *lark_http.Context) {
	c.Set("Access-Control-Allow-Origin", "*")
	c.Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,PATCH,OPTIONS")
	c.Set("Access-Control-Allow-Headers", "Authorization,Content-Type")
	if c.Req.Method == http.MethodOptions {
		c.SetStatus(http.StatusNoContent)
		c.Abort()
		return
	}
	c.Next()
}

func (a *App) authMiddleware(c *lark_http.Context) {
	token := auth.BearerToken(c.Get("Authorization"), c.Query("token"))
	username, ok := auth.ParseToken(a.cfg, token)
	if !ok {
		writeCode(c, code.TokenInvalid)
		c.Abort()
		return
	}
	c.State["username"] = username
	c.Next()
}

func usernameFrom(c *lark_http.Context) string {
	username, _ := c.State["username"].(string)
	return username
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type questionRequest struct {
	Question  string `json:"question"`
	ModelType string `json:"model_type"`
	SessionID string `json:"session_id"`
}

type sessionRequest struct {
	SessionID string `json:"session_id"`
}

func (a *App) login(c *lark_http.Context) {
	var req loginRequest
	if err := c.BindJSON(&req); err != nil || req.Username == "" || req.Password == "" {
		writeCode(c, code.ParamsInvalid)
		return
	}
	token, result := a.services.Login(c.Req.Context(), req.Username, req.Password)
	if result != code.OK {
		writeCode(c, result)
		return
	}
	writeSuccess(c, lark_http.H{"token": token})
}

func (a *App) register(c *lark_http.Context) {
	var req registerRequest
	if err := c.BindJSON(&req); err != nil || req.Email == "" || req.Password == "" {
		writeCode(c, code.ParamsInvalid)
		return
	}
	token, username, result := a.services.Register(c.Req.Context(), req.Email, req.Password)
	if result != code.OK {
		writeCode(c, result)
		return
	}
	writeSuccess(c, lark_http.H{"token": token, "username": username})
}

func (a *App) getUserSessions(c *lark_http.Context) {
	username := usernameFrom(c)
	key := sessionsCacheKey(username)
	var sessions interface{}
	if a.cacheGetJSON(c.Req.Context(), key, &sessions) {
		writeSuccess(c, lark_http.H{"sessions": sessions})
		return
	}
	typedSessions := a.services.Sessions(c.Req.Context(), username)
	a.cacheSetJSON(c.Req.Context(), key, typedSessions)
	writeSuccess(c, lark_http.H{"sessions": typedSessions})
}

func (a *App) createSessionAndSendMessage(c *lark_http.Context) {
	var req questionRequest
	if err := c.BindJSON(&req); err != nil || !isValidQuestion(req) {
		writeCode(c, code.ParamsInvalid)
		return
	}
	username := usernameFrom(c)
	sessionID, result := a.services.CreateSession(c.Req.Context(), username, req.Question)
	if result != code.OK {
		writeCode(c, result)
		return
	}
	a.invalidateSessionCaches(c.Req.Context(), username, sessionID)
	resp, err := a.rpc.Complete(c.Req.Context(), rpc_client.AIRequest{Username: username, SessionID: sessionID, Question: req.Question, ModelType: req.ModelType})
	if err != nil || code.Code(resp.Code) != code.OK {
		writeCode(c, code.ModelError)
		return
	}
	a.cacheDelete(c.Req.Context(), historyCacheKey(username, sessionID))
	writeSuccess(c, lark_http.H{"session_id": sessionID, "answer": resp.Answer})
}

func (a *App) createSessionAndSendMessageStream(c *lark_http.Context) {
	var req questionRequest
	if err := c.BindJSON(&req); err != nil || !isValidQuestion(req) {
		writeCode(c, code.ParamsInvalid)
		return
	}
	username := usernameFrom(c)
	sessionID, result := a.services.CreateSession(c.Req.Context(), username, req.Question)
	if result != code.OK {
		writeCode(c, result)
		return
	}
	a.invalidateSessionCaches(c.Req.Context(), username, sessionID)
	sse := c.SSE()
	sse.Event("session", sessionID)
	stream, err := a.rpc.CompleteStream(c.Req.Context(), rpc_client.AIRequest{Username: username, SessionID: sessionID, Question: req.Question, ModelType: req.ModelType})
	if err != nil {
		sse.Event("error", err.Error())
		sse.Done()
		return
	}
	for {
		var chunk rpc_client.AIStreamChunk
		if err := stream.Recv(&chunk); err != nil {
			if err != io.EOF {
				sse.Event("error", err.Error())
			}
			break
		}
		sse.Data(chunk.Content)
	}
	a.cacheDelete(c.Req.Context(), historyCacheKey(username, sessionID))
	sse.Done()
}

func (a *App) sendMessageToSession(c *lark_http.Context) {
	var req questionRequest
	if err := c.BindJSON(&req); err != nil || !isValidQuestionWithSession(req) {
		writeCode(c, code.ParamsInvalid)
		return
	}
	username := usernameFrom(c)
	resp, err := a.rpc.Complete(c.Req.Context(), rpc_client.AIRequest{Username: username, SessionID: req.SessionID, Question: req.Question, ModelType: req.ModelType})
	if err != nil || code.Code(resp.Code) != code.OK {
		writeCode(c, code.ModelError)
		return
	}
	a.cacheDelete(c.Req.Context(), historyCacheKey(username, req.SessionID))
	writeSuccess(c, lark_http.H{"answer": resp.Answer})
}

func (a *App) sendMessageStreamToSession(c *lark_http.Context) {
	var req questionRequest
	if err := c.BindJSON(&req); err != nil || !isValidQuestionWithSession(req) {
		writeCode(c, code.ParamsInvalid)
		return
	}
	username := usernameFrom(c)
	sse := c.SSE()
	stream, err := a.rpc.CompleteStream(c.Req.Context(), rpc_client.AIRequest{Username: username, SessionID: req.SessionID, Question: req.Question, ModelType: req.ModelType})
	if err != nil {
		sse.Event("error", err.Error())
		sse.Done()
		return
	}
	for {
		var chunk rpc_client.AIStreamChunk
		if err := stream.Recv(&chunk); err != nil {
			if err != io.EOF {
				sse.Event("error", err.Error())
			}
			break
		}
		sse.Data(chunk.Content)
	}
	a.cacheDelete(c.Req.Context(), historyCacheKey(username, req.SessionID))
	sse.Done()
}

func (a *App) getChatHistoryList(c *lark_http.Context) {
	var req sessionRequest
	if err := c.BindJSON(&req); err != nil || req.SessionID == "" {
		writeCode(c, code.ParamsInvalid)
		return
	}
	username := usernameFrom(c)
	key := historyCacheKey(username, req.SessionID)
	var history interface{}
	if a.cacheGetJSON(c.Req.Context(), key, &history) {
		writeSuccess(c, lark_http.H{"history": history})
		return
	}
	typedHistory, result := a.services.History(c.Req.Context(), username, req.SessionID)
	if result != code.OK {
		writeCode(c, result)
		return
	}
	a.cacheSetJSON(c.Req.Context(), key, typedHistory)
	writeSuccess(c, lark_http.H{"history": typedHistory})
}

func (a *App) uploadFile(c *lark_http.Context) {
	username := usernameFrom(c)
	if username == "" {
		writeCode(c, code.TokenInvalid)
		return
	}
	if err := c.Req.ParseMultipartForm(32 << 20); err != nil {
		writeCode(c, code.ParamsInvalid)
		return
	}
	file, header, err := c.Req.FormFile("file")
	if err != nil {
		writeCode(c, code.ParamsInvalid)
		return
	}
	defer file.Close()
	path, err := saveUpload(username, header, file)
	if err != nil {
		writeCode(c, code.ServerError)
		return
	}
	if indexErr := a.services.IndexFile(c.Req.Context(), username, path); indexErr != nil {
		log.Printf("index file for rag failed: %v", indexErr)
	}
	writeSuccess(c, lark_http.H{"filepath": path})
}

func saveUpload(username string, header *multipart.FileHeader, file multipart.File) (string, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".md" && ext != ".txt" && ext != ".json" {
		return "", fmt.Errorf("unsupported file type")
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	userDir := filepath.Join("uploads", username)
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return "", err
	}
	filename := fmt.Sprintf("%08x%s", crc32.ChecksumIEEE(data), ext)
	path := filepath.Join(userDir, filename)
	return path, os.WriteFile(path, data, 0o644)
}

func isValidQuestion(req questionRequest) bool {
	return req.Question != "" && isSupportedModelType(req.ModelType)
}

func isValidQuestionWithSession(req questionRequest) bool {
	return isValidQuestion(req) && req.SessionID != ""
}

func isSupportedModelType(modelType string) bool {
	return modelType == ai.ModelOllama || modelType == ai.ModelOllamaRAG
}

func sessionsCacheKey(username string) string {
	return "sessions:" + username
}

func historyCacheKey(username string, sessionID string) string {
	return "history:" + username + ":" + sessionID
}

func (a *App) cacheGetJSON(ctx context.Context, key string, out interface{}) bool {
	if a.cache == nil {
		return false
	}
	value, ok := a.cache.Get(ctx, key)
	if !ok {
		return false
	}
	return json.Unmarshal([]byte(value), out) == nil
}

func (a *App) cacheSetJSON(ctx context.Context, key string, value interface{}) {
	if a.cache == nil {
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	_ = a.cache.Set(ctx, key, string(data))
}

func (a *App) cacheDelete(ctx context.Context, key string) {
	if a.cache != nil {
		_ = a.cache.Delete(ctx, key)
	}
}

func (a *App) invalidateSessionCaches(ctx context.Context, username string, sessionID string) {
	a.cacheDelete(ctx, sessionsCacheKey(username))
	a.cacheDelete(ctx, historyCacheKey(username, sessionID))
}

func writeCode(c *lark_http.Context, result code.Code) {
	c.Status = http.StatusOK
	c.JSON(codeBody(result))
}

func writeSuccess(c *lark_http.Context, extra lark_http.H) {
	body := codeBody(code.OK)
	for key, value := range extra {
		body[key] = value
	}
	c.Status = http.StatusOK
	c.JSON(body)
}

func codeBody(result code.Code) lark_http.H {
	return lark_http.H{"code": result, "message": code.Message(result)}
}
