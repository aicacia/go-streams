package rtsp

import (
	"fmt"
	"log"
	"path"
	"sync"
	"time"

	"github.com/aicacia/pubsub"
	"github.com/aicacia/streams/app/config"
	"github.com/aicacia/streams/app/format"
	"github.com/aicacia/streams/app/services"
	"github.com/aicacia/streams/app/util"
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
		go startRecording(cameraId, recorder.viewer.Socket)
	}
}

func restartRecording(cameraId string) {
	stopRecording(cameraId)
	addRecorder(cameraId)
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

func GetRecordingFolderPath(cameraId string, t *time.Time) string {
	return path.Join(
		config.Config.Recordings.Folder,
		cameraId,
		fmt.Sprintf("%d/%d/%d/%d/%d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute()),
	)
}

func startRecording(cameraId string, packets chan *av.Packet) {
	var muxer *format.Muxer
	var nextMinute time.Time
	for packet := range packets {
		currentTime := GetPacketTime(packet)
	Muxer:
		if muxer == nil {
			var err error
			muxer, err = format.NewMuxer(
				GetRecordingFolderPath(cameraId, &currentTime),
			)
			if err != nil {
				log.Printf("%s: Failed to create raw muxer %s", cameraId, err)
				restartRecording(cameraId)
				return
			}
			err = muxer.WriteHeader(GetCurrentCodecs(cameraId))
			if err != nil {
				muxer.Close()
				log.Printf("%s: Failed to write codecs %s", cameraId, err)
				restartRecording(cameraId)
				return
			}
			nextMinute = util.TruncateToMinute(currentTime.Add(time.Minute))
		}
		if currentTime.After(nextMinute) {
			muxer.Close()
			muxer = nil
			goto Muxer
		} else if !IsRecording(cameraId) {
			muxer.Close()
			break
		}
		err := muxer.WritePacket(packet)
		if err != nil {
			log.Printf("%s: Failed to write packet %s", cameraId, err)
		}
	}
}

func runRecord(subscriber *pubsub.Subscriber[services.CameraEvent]) {
	defer subscriber.Close()

	for e := range subscriber.C {
		switch (*e).Type() {
		case services.CameraAdded:
			event := (*e).(*services.AddCameraEvent)
			if event.Camera.Recording {
				addRecorder(event.Camera.Id)
			}
		case services.CameraUpdated:
			event := (*e).(*services.UpdateCameraEvent)
			if event.Camera.Recording && event.PrevCamera != nil && !event.PrevCamera.Recording {
				addRecorder(event.Camera.Id)
			} else {
				removeRecorder(event.Camera.Id)
			}
		case services.CameraDeleted:
			event := (*e).(*services.DeleteCameraEvent)
			removeRecorder(event.Camera.Id)
		}
	}
}

func InitRecord() {
	subscriber := services.CameraEventPubSub.Subscribe()
	go runRecord(subscriber)
}
