package format

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/aicacia/streams/app/util"

	"github.com/deepch/vdk/av"
)

type Muxer struct {
	folder string
	files  []*os.File
}

func NewMuxer(folder string) (*Muxer, error) {
	if _, err := os.Stat(folder); os.IsNotExist(err) {
		err := os.MkdirAll(folder, os.ModePerm)
		if err != nil {
			return nil, err
		}
	}
	return &Muxer{folder: folder, files: nil}, nil
}

func (element *Muxer) WriteHeader(streams []av.CodecData) error {
	files := make([]*os.File, len(streams))

	for idx, stream := range streams {
		file, fileErr := os.Create(element.folder + fmt.Sprintf("/%d", idx))
		if fileErr != nil {
			return fileErr
		}
		streamBytes, toBytesErr := util.ToBytes([]av.CodecData{stream})
		if toBytesErr != nil {
			return toBytesErr
		}
		_, writeErr := file.Write(append(streamBytes, StartCode...))
		if writeErr != nil {
			return writeErr
		}
		files[idx] = file
	}
	element.files = files
	return nil

}

func (element *Muxer) WritePacket(pkt *av.Packet) (err error) {
	file := element.files[pkt.Idx]
	bytes := make([]byte, 0)

	if pkt.IsKeyFrame {
		bytes = append(bytes, 1)
	} else {
		bytes = append(bytes, 0)
	}

	duration := make([]byte, 8)
	binary.LittleEndian.PutUint64(duration, uint64(pkt.Duration))
	bytes = append(bytes, duration...)

	timestamp := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestamp, uint64(pkt.Time))
	bytes = append(bytes, timestamp...)

	bytes = append(bytes, pkt.Data...)
	bytes = append(bytes, StartCode...)
	_, err = file.Write(bytes)
	return
}

func (element *Muxer) Close() (err error) {
	for _, file := range element.files {
		err = file.Close()
	}
	return err
}
