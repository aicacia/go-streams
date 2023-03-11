package router

import (
	"github.com/aicacia/streams/app/controllers"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

type ApiRouter struct {
}

func (h ApiRouter) InstallRouter(app *fiber.App) {
	group := app.Group("", cors.New(cors.Config{
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, Forwarded",
	}))

	group.Get("/health", controllers.GetHealthCheck)
	group.Get("/version", controllers.GetVersion)
	group.Post("/config", controllers.PostRefreshConfig)

	cameras := group.Group("/cameras")
	cameras.Get("", controllers.GetCameras)
	cameras.Patch("", controllers.PostCreateCamera)

	cameras_by_id := cameras.Group("/:cameraId")
	cameras_by_id.Get("", controllers.GetCameraById)
	cameras_by_id.Patch("", controllers.PatchUpdateCamera)
	cameras_by_id.Put("", controllers.PatchUpdateCamera)
	cameras_by_id.Delete("", controllers.DeleteCamera)

	camera_live := cameras_by_id.Group("/live")
	camera_live.Get("/codecs", controllers.GetLiveCodecs)
	camera_live.Post("/sdp", controllers.PostLiveSdp)

	camera_playback := cameras_by_id.Group("/playback")
	camera_playback.Post("", controllers.PostCreatePlayback)

	playback := group.Group("/playback")
	playback_by_id := playback.Group("/:playbackId")
	playback_by_id.Get("/codecs", controllers.GetPlaybackCodecs)
	playback_by_id.Post("/sdp", controllers.PostPlaybackSdp)
}

func NewApiRouter() *ApiRouter {
	return &ApiRouter{}
}
