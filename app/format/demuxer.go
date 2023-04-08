package format

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aicacia/streams/app/util"
	"github.com/deepch/vdk/av"
)

type Demuxer struct {
	idx     int8
	folder  string
	codec   av.CodecData
	file    *os.File
	scanner *bufio.Scanner
}

const maxCapacity = 1024 * 1024 * 8

func NewDemuxer(folder string, idx int8) (*Demuxer, error) {
	var codec av.CodecData
	file, err := os.Open(folder + fmt.Sprintf("/%d", idx))
	if err != nil {
		return nil, err
	}
	scanner := util.NewDelimScanner(file, StartCode)
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	if scanner.Scan() {
		bytes := scanner.Bytes()
		var c []av.CodecData
		err := util.FromBytes(bytes, &c)
		if err != nil {
			return nil, err
		}
		codec = c[0]
	} else {
		return nil, scanner.Err()
	}

	return &Demuxer{
		idx:     idx,
		folder:  folder,
		file:    file,
		codec:   codec,
		scanner: scanner,
	}, nil
}

func (d *Demuxer) Codec() av.CodecData {
	return d.codec
}

func (d *Demuxer) Streams() ([]av.CodecData, error) {
	return []av.CodecData{d.codec}, nil
}

func (d *Demuxer) ReadPacket(direction int8) (*av.Packet, error) {
	if d.scanner.Scan() {
		scannerBytes := d.scanner.Bytes()

		if len(scannerBytes) > 0 {
			isKeyFrame := false
			if scannerBytes[0] == 1 {
				isKeyFrame = true
			}
			duration := time.Duration(int64(binary.LittleEndian.Uint64(scannerBytes[1:9])))

			timestamp := int64(binary.LittleEndian.Uint64(scannerBytes[9:17]))

			return &av.Packet{
				IsKeyFrame:      isKeyFrame,
				Idx:             d.idx,
				CompositionTime: 0,
				Time:            time.Duration(timestamp),
				Duration:        duration,
				Data:            scannerBytes[17:],
			}, nil
		}
	}
	return nil, io.EOF
}

func (d *Demuxer) Close() (err error) {
	return d.file.Close()
}
