package webserver

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/model/vo"
)

// corsMiddleware sets permissive CORS headers on every response, mirroring the
// behaviour of the previous gin CORS handler.
func corsMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		method := c.Method()
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE,UPDATE")
		c.Set("Access-Control-Allow-Headers", "Authorization, Content-Length, X-CSRF-Token, Token,session,X_Requested_With,Accept, Origin, Host, Connection, Accept-Encoding, Accept-Language,DNT, X-CustomHeader, Keep-Alive, User-Agent, X-Requested-With, If-Modified-Since, Cache-Control, Content-Type, Pragma,token,openid,opentoken")
		c.Set("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers,Cache-Control,Content-Language,Content-Type,Expires,Last-Modified,Pragma,FooBar")
		c.Set("Access-Control-Max-Age", "172800")
		c.Set("Access-Control-Allow-Credentials", "false")
		c.Set("Content-Type", "application/json")
		if method == http.MethodOptions {
			return c.Status(http.StatusOK).JSON(&vo.CodeMsg{})
		}

		// Continue processing the request.
		return c.Next()
	}
}
