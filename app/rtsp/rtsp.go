package rtsp

import (
	"encoding/gob"
	"log"

	"github.com/aicacia/streams/app/rtsp/cameras"
	"github.com/aicacia/streams/app/services"
	"github.com/deepch/vdk/codec"
	"github.com/deepch/vdk/codec/h264parser"
	"github.com/deepch/vdk/format"
)

func init() {
	format.RegisterAll()

	gob.Register(h264parser.CodecData{})
	gob.Register(codec.PCMUCodecData{})
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
