package main

import (
	"fmt"
	"log"
	"time"

	"github.com/aicacia/streams/app"
	"github.com/aicacia/streams/app/config"
	"github.com/aicacia/streams/app/rtsp"
	"github.com/aicacia/streams/app/services"
	_ "github.com/aicacia/streams/docs"
	"github.com/aicacia/streams/pkg/router"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

var (
	Version string = "0.0.0"
	Build   string = fmt.Sprint(time.Now().UnixMilli())
)

// @title Streams
// @version 0.0.0
// @description Streams API
// @termsOfService http://streams.aicacia.com/terms
// @contact.name Nathan Faucett
// @contact.email nathanfaucett@email.com
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @host localhost:8080
// @BasePath /
func main() {
	app.Version.Version = Version
	app.Version.Build = Build

	config.InitConfig()

	rtsp.InitClients()
	rtsp.InitRecord()

	services.InitCameras()

	// https://docs.gofiber.io/api/fiber#config
	app := fiber.New(fiber.Config{
		Prefork:       false,
		CaseSensitive: true,
		StrictRouting: true,
		ServerHeader:  "",
		AppName:       "",
	})
	app.Use(recover.New())
	app.Use(logger.New())
	app.Get("/dashboard", monitor.New())
	router.InstallRouter(app)

	log.Fatal(app.Listen(fmt.Sprintf("%s:%d", config.Config.Host, config.Config.Port)))
}
