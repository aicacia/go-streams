package rtsp

import (
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/aicacia/streams/app/config"
	"github.com/aicacia/streams/app/util"
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
	running   bool
	closed    bool
	socket    chan av.Packet
}

func NewPlayback(cameraId string, start *time.Time) (*uuid.UUID, error) {
	socket := make(chan av.Packet, socketChanSize)
	playback := PlaybackST{
		uuid:      uuid.New(),
		cameraId:  cameraId,
		direction: PlaybackForward,
		rate:      1.0,
		codecs:    nil,
		running:   true,
		closed:    false,
		socket:    socket,
	}
	playbacksMutex.Lock()
	playbacks[playback.uuid.String()] = &playback
	playbacksMutex.Unlock()
	var currentTime time.Time
	if start != nil {
		currentTime = *start
	}
	go playback_worker(playback.uuid.String(), cameraId, socket, currentTime)
	return &playback.uuid, nil
}

func playbackIsRunning(playbackId string) bool {
	playbacksMutex.RLock()
	defer playbacksMutex.RUnlock()
	if playback, ok := playbacks[playbackId]; ok && playback != nil {
		return playback.running
	} else {
		return false
	}
}

func playbackIsClosed(playbackId string) bool {
	playbacksMutex.RLock()
	defer playbacksMutex.RUnlock()
	if playback, ok := playbacks[playbackId]; ok && playback != nil {
		return playback.closed
	} else {
		return false
	}
}

func playbackSendQuit(playbackId string) {
	playbacksMutex.Lock()
	defer playbacksMutex.Unlock()
	if playback, ok := playbacks[playbackId]; ok && playback != nil {
		playback.closed = true
	}
}

func playbackSendQuitAndWait(playbackId string) {
	playbackSendQuit(playbackId)
	for {
		if !playbackIsRunning(playbackId) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func playbackStop(playbackId string) {
	playbacksMutex.Lock()
	defer playbacksMutex.Unlock()
	if playback, ok := playbacks[playbackId]; ok && playback != nil {
		playback.running = false
	}
}

func GetPlaybackCodecs(playbackId string) []av.CodecData {
	for i := 0; i < config.Config.RTSP.Connect.Timeout.Seconds*10; i++ {
		playbacksMutex.RLock()
		playback, ok := playbacks[playbackId]
		playbacksMutex.RUnlock()
		if ok && playback != nil && playback.codecs != nil {
			return playback.codecs
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

func PlaybackDelete(playbackId string) {
	playbackSendQuitAndWait(playbackId)
	playbacksMutex.Lock()
	delete(playbacks, playbackId)
	playbacksMutex.Unlock()
}

func GetPlaybackSocket(playbackId string) chan av.Packet {
	playbacksMutex.RLock()
	defer playbacksMutex.RUnlock()
	if playback, ok := playbacks[playbackId]; ok && playback != nil {
		return playback.socket
	}
	return nil
}

func setPlaybackCodecs(playbackId string, codecs []av.CodecData) {
	playbacksMutex.Lock()
	defer playbacksMutex.Unlock()
	if playback, ok := playbacks[playbackId]; ok && playback != nil {
		playback.codecs = codecs
	}
}

func getIdxCodec(codecs []av.CodecData) (int8, int8) {
	var videoIdx int8 = 0
	var audioIdx int8 = 0
	for index, codec := range codecs {
		if codec.Type().IsVideo() {
			videoIdx = int8(index)
		} else if codec.Type().IsAudio() {
			audioIdx = int8(index)
		}
	}
	return videoIdx, audioIdx
}

func playback_worker(playbackId, cameraId string, socket chan av.Packet, currentTime time.Time) {
	defer playbackStop(playbackId)
Outer:
	for {
		if playbackIsClosed(playbackId) {
			return
		}
		currentTimeToTheMinute := currentTime.Truncate(time.Duration(currentTime.Nanosecond()))
		folderPath := getFolderPath(cameraId, &currentTimeToTheMinute)
		recordingFilePath := getRecordingFilePath(folderPath, &currentTime)

		metaFilePath := getMetaFilePath(folderPath, &currentTime)
		metaFile, metaFileErr := os.Open(metaFilePath)
		if metaFileErr != nil {
			log.Printf("%s: Failed to open meta file %s", cameraId, metaFileErr)
			currentTime = currentTimeToTheMinute.Add(1 * time.Minute)
			continue
		}
		metaBytes, metaReadErr := io.ReadAll(metaFile)
		if metaReadErr != nil {
			log.Printf("%s: Failed to open meta file %s", cameraId, metaReadErr)
			currentTime = currentTimeToTheMinute.Add(1 * time.Minute)
			continue
		}
		var meta RecordingMetaST
		metaErr := util.FromBytes(metaBytes, &meta)
		if metaErr != nil {
			log.Printf("%s: Failed to read meta file %s", cameraId, metaErr)
			currentTime = currentTimeToTheMinute.Add(1 * time.Minute)
			continue
		}
		setPlaybackCodecs(playbackId, meta.Codecs)
		videoIdx, audioIdx := getIdxCodec(meta.Codecs)

		file, fileErr := os.Open(recordingFilePath)
		if fileErr != nil {
			log.Printf("%s: Failed to open recording file %s", cameraId, fileErr)
			currentTime = currentTimeToTheMinute.Add(1 * time.Minute)
			continue
		}
		scanner := NewPacketScanner(file)
		for {
			packet, err := PacketScannerRead(scanner)
			if err != nil {
				if err != io.EOF {
					log.Printf("%s: Failed read packet %s", cameraId, err)
					continue
				}
				continue Outer
			}
			packetTime := currentTimeToTheMinute.Add(packet.Duration)
			if packetTime.Before(currentTime) {
				continue Outer
			}
			socket <- *packet
			if packet.IsKeyFrame {
				if playbackIsClosed(playbackId) {
					return
				}
				if packet.Idx == videoIdx {
					time.Sleep(packet.Duration)
				}
			}
			if packet.Idx == int8(audioIdx) {
				time.Sleep(packet.Duration)
			}
		}
	}
}
