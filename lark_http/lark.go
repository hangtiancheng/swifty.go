package lark_http

import (
	"context"
	"html/template"
	"net/http"
)

type Engine struct {
	*RouterGroup
	router        *router
	groups        []*RouterGroup
	htmlTemplates *template.Template
	funcMap       template.FuncMap
	server        *http.Server
}

func New() *Engine {
	engine := &Engine{router: newRouter()}
	engine.RouterGroup = &RouterGroup{engine: engine}
	engine.groups = []*RouterGroup{engine.RouterGroup}
	return engine
}

func Default() *Engine {
	engine := New()
	engine.Use(Logger(), Recovery())
	return engine
}

func (engine *Engine) SetFuncMap(funcMap template.FuncMap) {
	engine.funcMap = funcMap
}

func (engine *Engine) LoadHTMLGlob(pattern string) {
	engine.htmlTemplates = template.Must(
		template.New("").Funcs(engine.funcMap).ParseGlob(pattern),
	)
}

func (engine *Engine) Run(addr string) error {
	engine.server = &http.Server{Addr: addr, Handler: engine}
	return engine.server.ListenAndServe()
}

func (engine *Engine) Shutdown(ctx context.Context) error {
	if engine.server == nil {
		return nil
	}
	return engine.server.Shutdown(ctx)
}

func (engine *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var middlewares []HandlerFunc
	for _, group := range engine.groups {
		if matchGroupPath(req.URL.Path, group.prefix) {
			middlewares = append(middlewares, group.middlewares...)
		}
	}
	c := newContext(w, req)
	c.handlers = middlewares
	c.engine = engine
	engine.router.handle(c)
	c.respond()
}
