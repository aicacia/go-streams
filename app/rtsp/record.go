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

func IsRecording(cameraId string) bool {
	recordingsMutex.RLock()
	defer recordingsMutex.RUnlock()
	if recorder, ok := recordings[cameraId]; ok && recorder != nil {
		return recorder.recording
	} else {
		return false
	}
}

func stopRecordingAndWait(cameraId string) {
	stopRecording(cameraId)
	for {
		if !IsRecording(cameraId) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func stopRecording(cameraId string) {
	recordingsMutex.Lock()
	defer recordingsMutex.Unlock()
	if recorder, ok := recordings[cameraId]; ok && recorder != nil {
		recorder.recording = false
	}
}

func removeRecorder(cameraId string) {
	stopRecordingAndWait(cameraId)
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

func getCodecsFilePath(folderPath string, t *time.Time) string {
	return path.Join(folderPath, fmt.Sprintf("%d.codecs", t.Minute()))
}

func getPacketFilePath(folderPath string, t *time.Time, idx int8) string {
	return path.Join(folderPath, fmt.Sprintf("%d.%d.packets", t.Minute(), idx))
}

type RecordPacket = struct {
	av.Packet
	RecordTime time.Time
}

func NewRecordPacket(packet av.Packet) RecordPacket {
	RecordTime := time.Now().UTC()
	return RecordPacket{
		packet,
		RecordTime,
	}
}

func startRecording(cameraId string, packets *chan av.Packet, codecs []av.CodecData) {
	for {
		if !IsRecording(cameraId) {
			return
		}
		start := time.Now().UTC()
		folderPath := getFolderPath(cameraId, &start)

		mkdirErr := os.MkdirAll(folderPath, os.ModePerm)
		if mkdirErr != nil {
			log.Printf("%s: Failed to create recording folder %s", cameraId, mkdirErr)
		}

		codecsFilePath := getCodecsFilePath(folderPath, &start)
		codecsFile, codecsFileErr := os.OpenFile(codecsFilePath, os.O_CREATE|os.O_WRONLY, 0600)
		if codecsFileErr != nil {
			log.Printf("%s: Failed to create recording file %s", cameraId, codecsFileErr)
		}
		codecs := GetCurrentCodecs(cameraId)
		codecsBytes, codecsErr := util.ToBytes(&codecs)
		if codecsErr != nil {
			log.Printf("%s: Failed to create codecs file %s", cameraId, codecsErr)
		}
		_, codecsFileWriteErr := codecsFile.Write(codecsBytes)
		if codecsFileWriteErr != nil {
			log.Printf("%s: Failed to create codecs file %s", cameraId, codecsFileWriteErr)
		}
		packetFiles, packetFilesErr := openPacketFilesAppend(folderPath, &start, codecs)
		if packetFilesErr != nil {
			log.Printf("%s: Failed to create packet files %s", cameraId, packetFilesErr)
		}
		start = time.Now().UTC()
		for packet := range *packets {
			recordPacket := NewRecordPacket(packet)
			err := writePacketFile(packetFiles, &recordPacket)
			if err != nil {
				log.Printf("%s: Failed to write packet %s", cameraId, err)
			}
			if time.Since(start) >= time.Minute {
				break
			}
		}
		codecsFile.Close()
		closePacketFiles(packetFiles)
	}
}

func openPacketFilesAppend(folderPath string, t *time.Time, codecs []av.CodecData) ([]*os.File, error) {
	var files []*os.File
	for index := range codecs {
		filePath := getPacketFilePath(folderPath, t, int8(index))
		file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func openPacketFilesRead(folderPath string, t *time.Time, codecs []av.CodecData) ([]*os.File, error) {
	var files []*os.File
	for index := range codecs {
		filePath := getPacketFilePath(folderPath, t, int8(index))
		file, err := os.Open(filePath)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func closePacketFiles(files []*os.File) {
	for _, file := range files {
		file.Close()
	}
}

func writePacketFile(files []*os.File, packet *RecordPacket) error {
	file := files[packet.Idx]
	bytes, bytesErr := util.ToBytes(packet)
	if bytesErr != nil {
		return bytesErr
	}
	bytes = append(bytes, PACKET_NEWLINE_BYTES...)
	_, writeErr := file.Write(bytes)
	if writeErr != nil {
		return writeErr
	}
	return nil
}

var PACKET_NEWLINE_BYTES = []byte{'.', '\n', '.', '\n', '.'}

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
	scanner.Buffer(make([]byte, 0, 8*1024), 1024*1024*1024)
	scanner.Split(scanLines)
	return scanner
}

func PacketScannerRead(scanner *PacketScanner) (*RecordPacket, error) {
	if scanner.Scan() {
		var packet RecordPacket
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

func runRecord(subscriber *pubsub.Subscriber[cameras.CameraEventST]) {
	defer subscriber.Close()

	for event := range subscriber.C {
		switch event.Type {
		case cameras.Added:
			addRecorder(event.Camera.Id)
		case cameras.Updated:
			if event.Camera.Recording && event.PrevCamera != nil && !event.PrevCamera.Recording {
				addRecorder(event.Camera.Id)
			} else {
				removeRecorder(event.Camera.Id)
			}
		case cameras.Deleted:
			removeRecorder(event.Camera.Id)
		}
	}
}

func InitRecord() {
	subscriber := cameras.EventPubSub.Subscribe()
	go runRecord(subscriber)
}
