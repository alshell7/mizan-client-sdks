// Package fiber adapts the framework-neutral Mizan webhook receiver to Go Fiber v2.
package fiber

import (
	"net/http"

	"github.com/alshell7/mizan-client-sdks/mizan-go"
	fiberlib "github.com/gofiber/fiber/v2"
)

// Middleware returns a Fiber handler that consumes both ledger and notification
// webhooks. It may be mounted on any POST route.
func Middleware(receiver mizan.WebhookReceiver) fiberlib.Handler {
	return func(c *fiberlib.Ctx) error {
		headers := make(http.Header)
		c.Context().Request.Header.VisitAll(func(name, value []byte) {
			headers.Add(string(name), string(value))
		})
		body := append([]byte(nil), c.Body()...)
		response := receiver.Receive(c.UserContext(), headers, body)
		for name, values := range response.Headers {
			for _, value := range values {
				c.Append(name, value)
			}
		}
		return c.Status(response.StatusCode).Send(response.Body)
	}
}

// Handler is an alias for Middleware for route-oriented code.
func Handler(receiver mizan.WebhookReceiver) fiberlib.Handler {
	return Middleware(receiver)
}
