package rtsp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"sync"
	"time"

	"github.com/aicacia/streams/app/config"
	"github.com/aicacia/streams/app/rtsp/cameras"
	"github.com/aicacia/streams/app/util"
	"github.com/aicacia/streams/pubsub"
	"github.com/deepch/vdk/av"
)

type recorderST struct {
	cameraId  string
	recording bool
	viewer    *ViewerST
}

var recordingsMutex sync.RWMutex
var recordings = make(map[string]*recorderST)

func addRecorder(cameraId string) {
	recordingsMutex.RLock()
	recorder, ok := recordings[cameraId]
	recordingsMutex.RUnlock()
	if !ok || recorder == nil {
		WaitForClient(cameraId)

		recorder = &recorderST{
			cameraId:  cameraId,
			recording: false,
			viewer:    AddViewer(cameraId),
		}
		recordingsMutex.Lock()
		recordings[cameraId] = recorder
		recordingsMutex.Unlock()
	}
	if recorder != nil && !recorder.recording {
		recorder.recording = true
		codecs := GetCodecs(cameraId)
		go startRecording(cameraId, &recorder.viewer.Socket, codecs)
	}
}

func removeRecorder(cameraId string) {
	recordingsMutex.Lock()
	delete(recordings, cameraId)
	recordingsMutex.Unlock()
}

func getFolderPath(cameraId string, t *time.Time) string {
	return path.Join(
		config.Config.Recordings.Folder,
		cameraId,
		fmt.Sprintf("%d/%d/%d/%d", t.Year(), t.Month(), t.Day(), t.Hour()),
	)
}

func getRecordingFilePath(folderPath string, t *time.Time) string {
	return path.Join(
		folderPath,
		fmt.Sprintf("%d.packets", t.Minute()),
	)
}

func getMetaFilePath(folderPath string, t *time.Time) string {
	return path.Join(
		folderPath,
		fmt.Sprintf("%d.meta", t.Minute()),
	)
}

type RecordingMetaST struct {
	Codecs []av.CodecData `json:"codecs"`
}

func startRecording(cameraId string, packets *chan av.Packet, codecs []av.CodecData) {
	for {
		start := time.Now().UTC()
		folderPath := getFolderPath(cameraId, &start)

		mkdirErr := os.MkdirAll(folderPath, os.ModePerm)
		if mkdirErr != nil {
			log.Printf("%s: Failed to create recording folder %s", cameraId, mkdirErr)
		}

		recordingFilePath := getRecordingFilePath(folderPath, &start)
		packetFile, packetFileErr := os.OpenFile(recordingFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if packetFileErr != nil {
			log.Printf("%s: Failed to create recording file %s", cameraId, packetFileErr)
		}

		metaFilePath := getMetaFilePath(folderPath, &start)
		metaFile, metaFileErr := os.OpenFile(metaFilePath, os.O_CREATE|os.O_WRONLY, 0600)
		if metaFileErr != nil {
			log.Printf("%s: Failed to create recording file %s", cameraId, metaFileErr)
		}
		codecs := GetCurrentCodecs(cameraId)
		metaBytes, metaErr := util.ToBytes(&RecordingMetaST{
			Codecs: codecs,
		})
		if metaErr != nil {
			log.Printf("%s: Failed to create meta file %s", cameraId, metaErr)
		}
		_, metaFileWriteErr := metaFile.Write(metaBytes)
		if metaFileWriteErr != nil {
			log.Printf("%s: Failed to create meta file %s", cameraId, metaFileWriteErr)
		}
		for packet := range *packets {
			bytes, bytesErr := packetToBytes(&packet)
			if bytesErr != nil {
				log.Printf("%s: Failed to convert packet to bytes %s", cameraId, bytesErr)
			}
			_, writeErr := packetFile.Write(bytes)
			if writeErr != nil {
				log.Printf("%s: Failed to write packet %s", cameraId, writeErr)
			}
			if time.Since(start) >= time.Minute {
				break
			}
		}
		packetFile.Close()
		metaFile.Close()
	}
}

var PACKET_NEWLINE_BYTES = []byte{'\n', '\n', '\n', '\n'}

func scanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i := 0; i+len(PACKET_NEWLINE_BYTES) <= len(data); {
		j := i + bytes.IndexByte(data[i:], PACKET_NEWLINE_BYTES[0])
		if j < i {
			break
		}
		if bytes.Equal(data[j+1:j+len(PACKET_NEWLINE_BYTES)], PACKET_NEWLINE_BYTES[1:]) {
			return j + len(PACKET_NEWLINE_BYTES), data[0:j], nil
		}
		i = j + 1
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

type PacketScanner = bufio.Scanner

func NewPacketScanner(reader io.Reader) *PacketScanner {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024)
	scanner.Split(scanLines)
	return scanner
}

func PacketScannerRead(scanner *PacketScanner) (*av.Packet, error) {
	if scanner.Scan() {
		var packet av.Packet
		err := util.FromBytes(scanner.Bytes(), &packet)
		if err != nil {
			return nil, err
		}
		return &packet, nil
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}

func packetToBytes(packet *av.Packet) ([]byte, error) {
	bytes, bytesErr := util.ToBytes(packet)
	if bytesErr != nil {
		return nil, bytesErr
	}
	bytes = append(bytes, PACKET_NEWLINE_BYTES...)
	return bytes, nil
}

func runRecord(subscriber *pubsub.Subscriber[cameras.CameraEventST]) {
	defer subscriber.Close()

	for event := range subscriber.C {
		switch event.Type {
		case cameras.Added:
			addRecorder(event.Camera.Id)
		case cameras.Updated:
		case cameras.Deleted:
			removeRecorder(event.Camera.Id)
		}
	}
}

func InitRecord() {
	subscriber := cameras.EventPubSub.Subscribe()
	go runRecord(subscriber)
}
