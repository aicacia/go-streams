package playback

import (
	"log"
	"sync"
	"time"

	"github.com/aicacia/streams/app/config"
	"github.com/aicacia/streams/app/rtsp"
	"github.com/aicacia/streams/app/util"
	"github.com/deepch/vdk/av"
	"github.com/google/uuid"
)

const (
	PlaybackBackward int8 = -1
	PlaybackForward  int8 = 1
)

const socketChanSize = 1024

var playbacksMutex sync.RWMutex
var playbacks = make(map[string]*PlaybackST)

type PlaybackST struct {
	uuid        uuid.UUID
	cameraId    string
	direction   int8
	rate        float32
	currentTime time.Time
	codecs      []av.CodecData
	running     bool
	closed      bool
	socket      chan *av.Packet
}

func NewPlayback(cameraId string, start *time.Time) (*uuid.UUID, error) {
	var currentTime time.Time
	if start != nil {
		currentTime = *start
	}
	socket := make(chan *av.Packet, socketChanSize)
	playback := PlaybackST{
		uuid:        uuid.New(),
		cameraId:    cameraId,
		direction:   PlaybackForward,
		rate:        1.0,
		currentTime: currentTime,
		codecs:      nil,
		running:     true,
		closed:      false,
		socket:      socket,
	}
	playbacksMutex.Lock()
	playbacks[playback.uuid.String()] = &playback
	playbacksMutex.Unlock()
	go playbackWorker(playback.uuid.String(), cameraId, socket, currentTime)
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
	log.Printf("%s: Send Delete Request", playbackId)
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

func GetPlaybackRate(playbackId string) float32 {
	playbacksMutex.RLock()
	defer playbacksMutex.RUnlock()
	if playback, ok := playbacks[playbackId]; ok && playback != nil {
		return playback.rate
	} else {
		return 1.0
	}
}

func setPlaybackRate(playbackId string, rate float32) {
	playbacksMutex.Lock()
	defer playbacksMutex.Unlock()
	if playback, ok := playbacks[playbackId]; ok && playback != nil {
		playback.rate = rate
	}
}

func GetPlaybackDirection(playbackId string) int8 {
	playbacksMutex.RLock()
	defer playbacksMutex.RUnlock()
	if p, ok := playbacks[playbackId]; ok && p != nil {
		return p.direction
	} else {
		return PlaybackForward
	}
}

func setPlaybackDirection(playbackId string, direction int8) {
	playbacksMutex.Lock()
	defer playbacksMutex.Unlock()
	if playback, ok := playbacks[playbackId]; ok && playback != nil {
		playback.direction = direction
	}
}

func GetPlaybackCurrentTime(playbackId string) *time.Time {
	playbacksMutex.RLock()
	defer playbacksMutex.RUnlock()
	if playback, ok := playbacks[playbackId]; ok && playback != nil {
		return &playback.currentTime
	} else {
		return nil
	}
}

func setPlaybackCurrentTime(playbackId string, currentTime time.Time) {
	playbacksMutex.Lock()
	defer playbacksMutex.Unlock()
	if playback, ok := playbacks[playbackId]; ok && playback != nil {
		playback.currentTime = currentTime
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

func setPlaybackCodecs(playbackId string, codecs []av.CodecData) {
	playbacksMutex.Lock()
	defer playbacksMutex.Unlock()
	if playback, ok := playbacks[playbackId]; ok && playback != nil {
		playback.codecs = codecs
	}
}

func GetPlaybackSocket(playbackId string) chan *av.Packet {
	playbacksMutex.RLock()
	defer playbacksMutex.RUnlock()
	if playback, ok := playbacks[playbackId]; ok && playback != nil {
		return playback.socket
	}
	return nil
}

func PlaybackDelete(playbackId string) {
	playbackSendQuitAndWait(playbackId)
	playbacksMutex.Lock()
	delete(playbacks, playbackId)
	playbacksMutex.Unlock()
	log.Printf("%s: Closed playback viewer", playbackId)
}

func playbackWorker(playbackId, cameraId string, socket chan *av.Packet, currentTime time.Time) {
	defer playbackStop(playbackId)
	for {
		if playbackIsClosed(playbackId) {
			log.Printf("%s: playback closed\n", playbackId)
			break
		}
		log.Printf("%s: playing %s", cameraId, currentTime)
		folder := rtsp.GetRecordingFolderPath(cameraId, &currentTime)
		player, err := NewPlayer(folder, &currentTime, GetPlaybackDirection(playbackId), GetPlaybackRate(playbackId))
		if err != nil {
			log.Printf("%s: failed to create demuxer %s", cameraId, err)
			if GetPlaybackDirection(playbackId) == PlaybackForward {
				currentTime = util.TruncateToMinute(currentTime.Add(time.Minute))
			} else {
				currentTime = util.TruncateToMinute(currentTime.Add(-time.Minute))
			}
			continue
		}
		setPlaybackCodecs(playbackId, player.Codecs())
		player.Start()
		var packetTime time.Time
		stream := player.Stream()
		for {
			if packet, ok := <-stream; ok {
				packetTime = rtsp.GetPacketTime(packet)
				socket <- packet
			}
			if playbackIsClosed(playbackId) {
				player.Close()
				break
			}
			if player.IsClosed() {
				break
			}
		}
		log.Printf("%s: done with %s", cameraId, currentTime)
		currentTime = packetTime
		if currentTime.Equal(packetTime) {
			if GetPlaybackDirection(playbackId) == PlaybackForward {
				currentTime = util.TruncateToMinute(currentTime.Add(time.Minute))
			} else {
				currentTime = util.TruncateToMinute(currentTime.Add(-time.Minute))
			}
		}
	}
}
