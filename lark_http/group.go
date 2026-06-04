package lark_http

import (
	"log"
	"net/http"
	"path"
	"strings"
)

type Router struct {
	prefix      string
	middlewares []Middleware
	parent      *Router
	app         *Application
}

func (r *Router) Router(prefix string) *Router {
	newRouter := &Router{
		prefix: r.prefix + prefix,
		parent: r,
		app:    r.app,
	}
	r.app.routers = append(r.app.routers, newRouter)
	return newRouter
}

func (r *Router) Use(middlewares ...Middleware) {
	r.middlewares = append(r.middlewares, middlewares...)
}

func (r *Router) addRoute(method string, comp string, handler Middleware) {
	pattern := r.prefix + comp
	log.Printf("Route %4s - %s", method, pattern)
	r.app.router.addRoute(method, pattern, handler)
}

func (r *Router) Get(pattern string, handler Middleware) {
	r.addRoute("GET", pattern, handler)
}

func (r *Router) Post(pattern string, handler Middleware) {
	r.addRoute("POST", pattern, handler)
}

func (r *Router) Put(pattern string, handler Middleware) {
	r.addRoute("PUT", pattern, handler)
}

func (r *Router) Delete(pattern string, handler Middleware) {
	r.addRoute("DELETE", pattern, handler)
}

func (r *Router) Patch(pattern string, handler Middleware) {
	r.addRoute("PATCH", pattern, handler)
}

func (r *Router) Head(pattern string, handler Middleware) {
	r.addRoute("HEAD", pattern, handler)
}

func (r *Router) Options(pattern string, handler Middleware) {
	r.addRoute("OPTIONS", pattern, handler)
}

func (r *Router) All(pattern string, handler Middleware) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	for _, m := range methods {
		r.addRoute(m, pattern, handler)
	}
}

func (r *Router) Static(relativePath string, root string) {
	handler := r.createStaticHandler(relativePath, http.Dir(root))
	urlPattern := path.Join(relativePath, "/*filepath")
	r.Get(urlPattern, handler)
}

func (r *Router) createStaticHandler(relativePath string, fs http.FileSystem) Middleware {
	absolutePath := path.Join(r.prefix, relativePath)
	fileServer := http.StripPrefix(absolutePath, http.FileServer(fs))
	return func(ctx *Context, next func()) {
		file := ctx.Param("filepath")
		if _, err := fs.Open(file); err != nil {
			ctx.Status = http.StatusNotFound
			return
		}
		ctx.flushed = true
		fileServer.ServeHTTP(ctx.Writer, ctx.Request)
	}
}

func matchRouterPath(requestPath, routerPrefix string) bool {
	if routerPrefix == "" || routerPrefix == "/" {
		return true
	}
	if !strings.HasPrefix(requestPath, routerPrefix) {
		return false
	}
	return len(requestPath) == len(routerPrefix) || requestPath[len(routerPrefix)] == '/'
}
