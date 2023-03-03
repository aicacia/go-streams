package rtsp

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/deepch/vdk/av"
	"github.com/google/uuid"
)

const (
	PlaybackBackward = iota
	PlaybackForward
)

const socketChanSize = 1000

var playbacksMutex sync.RWMutex
var playbacks = make(map[string]*PlaybackST)

type PlaybackST struct {
	uuid      uuid.UUID
	cameraId  string
	direction int
	rate      float32
	codecs    []av.CodecData
	socket    chan av.Packet
}

func NewPlayback(cameraId string, start *time.Time) (*uuid.UUID, error) {
	playback := PlaybackST{
		uuid:      uuid.New(),
		cameraId:  cameraId,
		direction: PlaybackForward,
		rate:      1.0,
		codecs:    nil,
		socket:    make(chan av.Packet, socketChanSize),
	}
	playbacksMutex.Lock()
	playbacks[playback.uuid.String()] = &playback
	playbacksMutex.Unlock()
	var currentTime time.Time
	if start != nil {
		currentTime = *start
	}
	go playback_worker(&playback, currentTime)
	return &playback.uuid, nil
}

func playback_worker(playback *PlaybackST, currentTime time.Time) {
	for {
		currentTimeToTheMinute := currentTime.Truncate(time.Duration(currentTime.Nanosecond()))
		folderPath := getFolderPath(playback.cameraId, &currentTimeToTheMinute)
		recordingFilePath := getRecordingFilePath(folderPath, &currentTime)
		metaFilePath := getMetaFilePath(folderPath, &currentTime)
		file, file_err := os.Open(recordingFilePath)
		if file_err != nil {
			log.Printf("%s: Failed to open recording file %s", playback.cameraId, file_err)
			currentTime = currentTimeToTheMinute.Add(1 * time.Minute)
			continue
		}
		meta_file, meta_file_err := os.Open(metaFilePath)
		if meta_file_err != nil {
			log.Printf("%s: Failed to open recording file %s", playback.cameraId, meta_file_err)
			continue
		}
		meta_bytes, meta_read_err := io.ReadAll(meta_file)
		if meta_read_err != nil {
			log.Printf("%s: Failed to open meta file %s", playback.cameraId, meta_read_err)
			continue
		}
		var meta RecordingMetaST
		meta_json_err := json.Unmarshal(meta_bytes, &meta)
		if meta_json_err != nil {
			log.Printf("%s: Failed to read meta file json %s", playback.cameraId, meta_json_err)
			continue
		}
		playback.codecs = meta.Codecs
		readSeeker := io.ReadSeeker(file)
		for {
			packet, err := ReadPacket(readSeeker)
			if err != nil {
				log.Printf("%s: Failed read packet %s", playback.cameraId, err)
			}
			packetTime := currentTimeToTheMinute.Add(packet.Time)
			if packetTime.Before(currentTime) {
				continue
			}
			playback.socket <- *packet
			if packet.IsKeyFrame {
				time.Sleep(packet.Duration)
			}
		}
	}
}
