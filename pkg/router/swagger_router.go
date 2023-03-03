package router

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/swagger"
)

type SwaggerRouter struct {
}

func (h SwaggerRouter) InstallRouter(app *fiber.App) {
	app.Get("/swagger/*", swagger.HandlerDefault)
}

func NewSwaggerRouter() *SwaggerRouter {
	return &SwaggerRouter{}
}
