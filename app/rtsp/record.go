package rtsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/aicacia/streams/app/config"
	"github.com/aicacia/streams/app/rtsp/cameras"
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
		mkdir_err := os.MkdirAll(folderPath, os.ModePerm)
		if mkdir_err != nil {
			log.Printf("%s: Failed to create recording folder %s", cameraId, mkdir_err)
		}
		recordingFilePath := getRecordingFilePath(folderPath, &start)
		packet_file, packet_file_err := os.OpenFile(recordingFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0700)
		if packet_file_err != nil {
			log.Printf("%s: Failed to create recording file %s", cameraId, packet_file_err)
		}
		metaFilePath := getMetaFilePath(folderPath, &start)
		meta_file, meta_file_err := os.OpenFile(metaFilePath, os.O_CREATE|os.O_WRONLY, 0700)
		if meta_file_err != nil {
			log.Printf("%s: Failed to create recording file %s", cameraId, meta_file_err)
		}
		codecs := GetCurrentCodecs(cameraId)
		meta_json_bytes, meta_json_err := json.Marshal(&RecordingMetaST{
			Codecs: codecs,
		})
		if meta_json_err != nil {
			log.Printf("%s: Failed to create meta file %s", cameraId, meta_json_err)
		}
		_, meta_file_write_err := meta_file.Write(meta_json_bytes)
		if meta_file_write_err != nil {
			log.Printf("%s: Failed to create meta file %s", cameraId, meta_file_write_err)
		}
		for packet := range *packets {
			_, write_err := packet_file.Write(packetToLine(&packet))
			if write_err != nil {
				log.Printf("%s: Failed to write packet %s", cameraId, write_err)
			}
			if time.Since(start) >= time.Minute {
				break
			}
		}
		packet_file.Close()
		meta_file.Close()
	}
}

var PACKET_NEWLINE_BYTES = []byte{89, 50, 78, 47}

func packetToLine(packet *av.Packet) []byte {
	bytes := ([]byte)(fmt.Sprintf("%v,%d,%d,%d,", packet.IsKeyFrame, packet.Idx, packet.Time, packet.Duration))
	padSize := len(packet.Data) % 4
	if padSize != 0 {
		tmp := make([]byte, len(packet.Data)+padSize)
		copy(tmp, packet.Data)
		bytes = append(bytes, byte(padSize))
		bytes = append(bytes, tmp...)
	} else {
		bytes = append(bytes, packet.Data...)
	}
	bytes = append(bytes, PACKET_NEWLINE_BYTES...)
	return bytes
}
func ReadPacket(reader io.Reader) (*av.Packet, error) {
	start := time.Now()

	isKeyFrameBytes, err := readBytesUntil(reader, ',')
	if err != nil {
		return nil, err
	}
	idxBytes, err := readBytesUntil(reader, ',')
	if err != nil {
		return nil, err
	}
	timeBytes, err := readBytesUntil(reader, ',')
	if err != nil {
		return nil, err
	}
	durationBytes, err := readBytesUntil(reader, ',')
	if err != nil {
		return nil, err
	}
	data, err := readPacketData(reader)
	if err != nil {
		return nil, err
	}

	isKeyFrameBool, err := strconv.ParseBool(string(isKeyFrameBytes))
	if err != nil {
		return nil, err
	}
	idxInt, err := strconv.Atoi(string(idxBytes))
	if err != nil {
		return nil, err
	}
	timeInt64, err := strconv.ParseInt(string(timeBytes), 10, 64)
	if err != nil {
		return nil, err
	}
	durationInt64, err := strconv.ParseInt(string(durationBytes), 10, 64)
	if err != nil {
		return nil, err
	}

	return &av.Packet{
		IsKeyFrame:      isKeyFrameBool,
		Idx:             int8(idxInt),
		CompositionTime: time.Since(start),
		Time:            time.Duration(timeInt64),
		Duration:        time.Duration(durationInt64),
		Data:            data,
	}, nil
}

func readBytesUntil(reader io.Reader, stopCh byte) ([]byte, error) {
	var b = make([]byte, 1)
	var bytes []byte
	for {
		_, err := reader.Read(b)
		if err != nil {
			return nil, err
		}
		ch := b[0]
		if ch == stopCh {
			break
		} else {
			bytes = append(bytes, ch)
		}
	}
	return bytes, nil
}

func readPacketData(reader io.Reader) ([]byte, error) {
	var data []byte
	read_bytes := make([]byte, 4)
	for {
		_, err := reader.Read(read_bytes)
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return nil, err
			}
		} else if bytes.Equal(read_bytes, PACKET_NEWLINE_BYTES) {
			padSize := int(data[len(data)-5])
			if padSize != 0 {
				tmp := make([]byte, len(data)-padSize-1)
				copy(tmp, data)
				data = tmp
			}
		} else {
			data = append(data, read_bytes...)
		}
	}
	return data, nil
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
