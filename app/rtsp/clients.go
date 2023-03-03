package rtsp

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/aicacia/streams/app/config"
	"github.com/aicacia/streams/app/models"
	"github.com/aicacia/streams/app/rtsp/cameras"
	"github.com/aicacia/streams/pubsub"
	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/codec/h264parser"
	"github.com/deepch/vdk/format/rtspv2"
	"github.com/google/uuid"
)

var (
	ErrorRTSPClientExitRtspDisconnect = errors.New("client exit rtsp disconnect")
	ErrorRTSPClientNoClient           = errors.New("no client exists")
)

var clientsMutex sync.RWMutex
var clients = make(map[string]*clientST)

type clientST struct {
	CameraId   string
	Url        string
	Running    bool
	RTSPClient *rtspv2.RTSPClient
	Closed     bool
	CloseCh    chan bool
	Codecs     []av.CodecData
	Viewers    map[string]bool
}

func IsCameraStreaming(cameraId string) bool {
	clientsMutex.RLock()
	defer clientsMutex.RUnlock()
	if client, ok := clients[cameraId]; ok && client != nil {
		return client.RTSPClient != nil
	} else {
		return false
	}
}

func runIfNotRunning(camera *models.CameraST) {
	clientsMutex.RLock()
	client, ok := clients[camera.Id]
	clientsMutex.RUnlock()
	if !ok {
		client = &clientST{
			CameraId: camera.Id,
			Url:      camera.RtspUrl,
			Running:  false,
			Closed:   false,
			CloseCh:  make(chan bool, 1),
			Viewers:  make(map[string]bool),
		}
		clientsMutex.Lock()
		clients[camera.Id] = client
		clientsMutex.Unlock()
	}
	if !client.Running {
		client.Running = true
		client.Closed = false
		for len(client.CloseCh) > 0 {
			<-client.CloseCh
		}
		log.Printf("%s: Starting %s\n", camera.Id, camera.RtspUrl)
		go worker_loop(camera.Id, camera.RtspUrl)
	}
}

func clientSendQuit(cameraId string) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	if client, ok := clients[cameraId]; ok && client != nil {
		client.Closed = true
		client.CloseCh <- true
	}
}

func clientDelete(cameraId string) {
	clientSendQuitAndWait(cameraId)
	clientsMutex.Lock()
	delete(clients, cameraId)
	clientsMutex.Unlock()
}

func clientStop(cameraId string) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	if client, ok := clients[cameraId]; ok && client != nil {
		client.Running = false
	}
}

func clientSetRTSPClient(cameraId string, rtsp_client *rtspv2.RTSPClient) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	if client, ok := clients[cameraId]; ok && client != nil {
		client.RTSPClient = rtsp_client
	}
}

func clientSendQuitAndWait(cameraId string) {
	clientSendQuit(cameraId)
	for {
		if !clientIsRunning(cameraId) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func clientIsRunning(cameraId string) bool {
	clientsMutex.RLock()
	defer clientsMutex.RUnlock()
	if client, ok := clients[cameraId]; ok && client != nil {
		return client.Running
	} else {
		return false
	}
}

func clientIsClosed(cameraId string) bool {
	clientsMutex.RLock()
	defer clientsMutex.RUnlock()
	if client, ok := clients[cameraId]; ok && client != nil {
		return client.Closed
	} else {
		return false
	}
}

func clientSwap(camera *models.CameraST, prev_camera *models.CameraST) {
	if camera.RtspUrl != prev_camera.RtspUrl {
		log.Printf("%s: RTSP Url changed %s\n", prev_camera.RtspUrl, camera.RtspUrl)
		clientSendQuitAndWait(camera.Id)
	}
	runIfNotRunning(camera)
}

func clientCodecsAdd(cameraId string, codecs []av.CodecData) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	if client, ok := clients[cameraId]; ok && client != nil {
		client.Codecs = codecs
	}
}

func WaitForClient(cameraId string) {
	for {
		clientsMutex.RLock()
		client, ok := clients[cameraId]
		clientsMutex.RUnlock()
		if ok && client != nil && client.Running {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func GetCurrentCodecs(cameraId string) []av.CodecData {
	clientsMutex.RLock()
	defer clientsMutex.RUnlock()
	if client, ok := clients[cameraId]; ok && client != nil && client.Codecs != nil {
		valid_codecs := len(client.Codecs) > 0
		for _, codec := range client.Codecs {
			if codec.Type() == av.H264 {
				codecVideo, ok := codec.(h264parser.CodecData)
				if !ok || codecVideo.SPS() == nil || codecVideo.PPS() == nil || len(codecVideo.SPS()) <= 0 || len(codecVideo.PPS()) <= 0 {
					valid_codecs = false
					break
				}
			}
		}
		if valid_codecs {
			return client.Codecs
		}
	}
	return nil
}

func GetCodecs(cameraId string) []av.CodecData {
	for i := 0; i < config.Config.RTSP.Connect.Timeout.Seconds*10; i++ {
		clientsMutex.RLock()
		client, ok := clients[cameraId]
		clientsMutex.RUnlock()
		if !ok || client == nil {
			log.Printf("%s: No client\n", client.Url)
			return nil
		}
		if client.Codecs != nil {
			valid_codecs := len(client.Codecs) > 0
			for _, codec := range client.Codecs {
				if codec.Type() == av.H264 {
					codecVideo, ok := codec.(h264parser.CodecData)
					if !ok || codecVideo.SPS() == nil || codecVideo.PPS() == nil || len(codecVideo.SPS()) <= 0 || len(codecVideo.PPS()) <= 0 {
						log.Printf("%s: Bad Video Codec SPS or PPS Wait\n", client.Url)
						valid_codecs = false
						break
					}
				}
			}
			if valid_codecs {
				log.Printf("%s: Ok Video Ready to play\n", client.Url)
				return client.Codecs
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

var viewersMutex sync.RWMutex
var viewers = make(map[string]*ViewerST)

const viewerChanSize = 1000

type ViewerST struct {
	Uuid   uuid.UUID
	Socket chan av.Packet
}

func AddViewer(cameraId string) *ViewerST {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	if client, ok := clients[cameraId]; ok && client != nil {
		viewer := ViewerST{
			Uuid:   uuid.New(),
			Socket: make(chan av.Packet, viewerChanSize),
		}
		uuid := viewer.Uuid.String()
		client.Viewers[uuid] = true
		viewersMutex.Lock()
		viewers[uuid] = &viewer
		viewersMutex.Unlock()
		return &viewer
	}
	return nil
}

func DeleteViewer(cameraId string, uuid *uuid.UUID) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	if client, ok := clients[cameraId]; ok && client != nil {
		uuidString := uuid.String()
		delete(client.Viewers, uuidString)
		viewersMutex.Lock()
		delete(viewers, uuidString)
		viewersMutex.Unlock()
	}
}

func (v *ViewerST) cast(packet av.Packet) {
	if len(v.Socket) < cap(v.Socket) {
		v.Socket <- packet
	}
}

func cast(cameraId string, packet av.Packet) {
	clientsMutex.RLock()
	client, ok := clients[cameraId]
	clientsMutex.RUnlock()
	if ok && client != nil {
		for id := range client.Viewers {
			viewersMutex.RLock()
			viewer, ok := viewers[id]
			viewersMutex.RUnlock()
			if ok && viewer != nil {
				viewer.cast(packet)
			}
		}
	}
}

const max_wait_s = time.Duration(30) * time.Second

func worker_loop(cameraId, url string) {
	defer clientStop(cameraId)
	wait_s := time.Duration(1) * time.Second
	for {
		closed, err := worker(cameraId, url)
		if err != nil {
			log.Printf("%s: Error %s\n", url, err)
		}
		if closed || clientIsClosed(cameraId) {
			log.Printf("%s: Closed\n", url)
			break
		}
		log.Printf("%s: Waiting %s\n", cameraId, wait_s)
		time.Sleep(wait_s)
		if wait_s < max_wait_s {
			wait_s = wait_s * 2
		}
	}
}

func worker(cameraId, url string) (bool, error) {
	rtsp_client, err := rtspv2.Dial(rtspv2.RTSPClientOptions{
		URL:              url,
		DisableAudio:     false,
		DialTimeout:      time.Duration(config.Config.RTSP.Connect.Timeout.Seconds) * time.Second,
		ReadWriteTimeout: time.Duration(config.Config.RTSP.IO.Timeout.Seconds) * time.Second,
		Debug:            config.Config.RTSP.Debug,
	})
	if err != nil {
		return false, err
	}
	defer rtsp_client.Close()

	clientSetRTSPClient(cameraId, rtsp_client)
	defer clientSetRTSPClient(cameraId, nil)

	if rtsp_client.CodecData != nil {
		log.Printf("%s: Codecs: %d\n", url, len(rtsp_client.CodecData))
		clientCodecsAdd(cameraId, rtsp_client.CodecData)
	}
	clientsMutex.RLock()
	client, ok := clients[cameraId]
	clientsMutex.RUnlock()
	if !ok || client == nil {
		return true, ErrorRTSPClientNoClient
	}
	quitCh := &client.CloseCh
	for {
		select {
		case quit := <-*quitCh:
			log.Printf("%s: Camera kill signal=%v\n", url, quit)
			return true, nil
		case signals := <-rtsp_client.Signals:
			log.Printf("%s: Got a signal from RTSPClient.Signals\n", url)
			switch signals {
			case rtspv2.SignalCodecUpdate:
				log.Printf("%s: rtspv2.SignalCodecUpdate, Codecs: %d\n", url, len(rtsp_client.CodecData))
				clientCodecsAdd(cameraId, rtsp_client.CodecData)
			case rtspv2.SignalStreamRTPStop:
				log.Printf("%s: rtspv2.SignalClientRTPStop\n", url)
				return false, ErrorRTSPClientExitRtspDisconnect
			}
		case packetAV := <-rtsp_client.OutgoingPacketQueue:
			cast(cameraId, *packetAV)
		}
	}
}

func runClients(subscriber *pubsub.Subscriber[cameras.CameraEventST]) {
	defer subscriber.Close()

	for event := range subscriber.C {
		switch event.Type {
		case cameras.Added:
			runIfNotRunning(event.Camera)
		case cameras.Updated:
			if event.Camera.Disabled {
				clientDelete(event.Camera.Id)
			} else if event.PrevCamera != nil {
				clientSwap(event.Camera, event.PrevCamera)
			} else {
				runIfNotRunning(event.Camera)
			}
		case cameras.Deleted:
			clientDelete(event.Camera.Id)
		}
	}
}

func InitClients() {
	subscriber := cameras.EventPubSub.Subscribe()
	go runClients(subscriber)
}
