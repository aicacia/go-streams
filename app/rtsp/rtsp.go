package rtsp

import (
	"log"

	"github.com/aicacia/streams/app/rtsp/cameras"
	"github.com/aicacia/streams/app/services"
	"github.com/deepch/vdk/format"
)

func init() {
	format.RegisterAll()
}

func InitCameras() {
	allCameras, err := services.ListCameras()
	if err != nil {
		log.Fatal(err)
	}
	for _, camera := range allCameras {
		cameras.AddCamera(camera)
	}
}
