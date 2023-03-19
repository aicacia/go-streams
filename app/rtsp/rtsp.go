package rtsp

import (
	"encoding/gob"

	"github.com/deepch/vdk/codec"
	"github.com/deepch/vdk/codec/h264parser"
	"github.com/deepch/vdk/codec/h265parser"
	"github.com/deepch/vdk/format"
)

func init() {
	format.RegisterAll()

	gob.Register(h264parser.CodecData{})
	gob.Register(h265parser.CodecData{})
	gob.Register(codec.PCMUCodecData{})
}
