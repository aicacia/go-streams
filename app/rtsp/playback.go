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
	PlaybackBackward int8 = iota
	PlaybackForward
)

const socketChanSize = 1000

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
	socket      chan av.Packet
}

func NewPlayback(cameraId string, start *time.Time) (*uuid.UUID, error) {
	var currentTime time.Time
	if start != nil {
		currentTime = *start
	}
	socket := make(chan av.Packet, socketChanSize)
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
	if playback, ok := playbacks[playbackId]; ok && playback != nil {
		return playback.direction
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

func GetPlaybackSocket(playbackId string) chan av.Packet {
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

func playbackWorker(playbackId, cameraId string, socket chan av.Packet, currentTime time.Time) {
	defer playbackStop(playbackId)
	for {
		if playbackIsClosed(playbackId) {
			return
		}
		log.Printf("%s: playing %s", cameraId, currentTime)
		folderPath := getFolderPath(cameraId, &currentTime)

		codecsFilePath := getCodecsFilePath(folderPath, &currentTime)
		codecsFile, codecsFileErr := os.Open(codecsFilePath)
		if codecsFileErr != nil {
			log.Printf("%s: Failed to open codecs file %s", cameraId, codecsFileErr)
			currentTime = currentTime.Truncate(time.Duration(currentTime.Nanosecond())).Add(time.Minute)
			continue
		}
		codecsBytes, codecsReadErr := io.ReadAll(codecsFile)
		if codecsReadErr != nil {
			log.Printf("%s: Failed to read codecs file %s", cameraId, codecsReadErr)
			currentTime = currentTime.Truncate(time.Duration(currentTime.Nanosecond())).Add(time.Minute)
			continue
		}
		var codecs []av.CodecData
		codecsErr := util.FromBytes(codecsBytes, &codecs)
		if codecsErr != nil {
			log.Printf("%s: Failed to parse codecs file %s", cameraId, codecsErr)
			currentTime = currentTime.Truncate(time.Duration(currentTime.Nanosecond())).Add(time.Minute)
			continue
		}
		setPlaybackCodecs(playbackId, codecs)

		packetFiles, packetFilesErr := openPacketFilesRead(folderPath, &currentTime, codecs)
		if packetFilesErr != nil {
			log.Printf("%s: Failed to open recording file %s", cameraId, packetFilesErr)
			currentTime = currentTime.Truncate(time.Duration(currentTime.Nanosecond())).Add(time.Minute)
			continue
		}
		var wait sync.WaitGroup
		for index, packetFile := range packetFiles {
			wait.Add(1)
			go packetWriter(cameraId, playbackId, codecs[index].Type().IsAudio(), packetFile, &currentTime, socket, &wait)
		}
		wait.Wait()
		currentTime = *GetPlaybackCurrentTime(playbackId)
	}
}

func packetWriter(
	cameraId, playbackId string,
	isAudio bool,
	file *os.File,
	currentTime *time.Time,
	socket chan av.Packet,
	wait *sync.WaitGroup,
) {
	scanner := NewPacketScanner(file)
	initialPacket := false
	for {
		packet, err := PacketScannerRead(scanner)
		if err != nil {
			if err != io.EOF {
				log.Printf("%s: Failed packet %s", cameraId, err)
				continue
			} else {
				break
			}
		}
		if !initialPacket {
			if isAudio {
				initialPacket = true
			} else if packet.IsKeyFrame {
				initialPacket = true
			} else {
				time.Sleep(time.Duration(float32(packet.Duration) / GetPlaybackRate(playbackId)).Abs())
				continue
			}
		}
		if GetPlaybackDirection(playbackId) == PlaybackForward {
			if packet.RecordTime.Before(*currentTime) {
				continue
			}
		} else {
			if packet.RecordTime.After(*currentTime) {
				continue
			}
		}
		socket <- packet.Packet
		if !isAudio {
			setPlaybackCurrentTime(playbackId, packet.RecordTime)
		}
		if packet.IsKeyFrame || isAudio {
			if playbackIsClosed(playbackId) {
				break
			}
		}
		time.Sleep(time.Duration(float32(packet.Duration) / GetPlaybackRate(playbackId)).Abs())
	}
	wait.Done()
}
