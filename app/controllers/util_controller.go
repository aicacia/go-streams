package controllers

import (
	"log"
	"net/http"

	"github.com/aicacia/streams/app"
	"github.com/aicacia/streams/app/config"
	"github.com/aicacia/streams/app/models"
	"github.com/gofiber/fiber/v2"
)

func GetHealthCheck(c *fiber.Ctx) error {
	c.JSON(http.StatusOK)
	return c.JSON(models.HealthCheckST{
		Ok: true,
	})
}

func GetVersion(c *fiber.Ctx) error {
	c.JSON(http.StatusOK)
	return c.JSON(app.Version)
}

func PostRefreshConfig(c *fiber.Ctx) error {
	err := config.RefreshConfig()
	if err != nil {
		log.Println(err)
		c.Status(http.StatusInternalServerError)
		return c.JSON(models.ResponseErrorST{
			Error: "Error refreshing config",
		})
	}
	c.Status(http.StatusNoContent)
	return c.Send(nil)
}
