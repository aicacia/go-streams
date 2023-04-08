package rtsp

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/aicacia/pubsub"
	"github.com/aicacia/streams/app/config"
	"github.com/aicacia/streams/app/models"
	"github.com/aicacia/streams/app/services"
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
	cameraId string
	url      string
	running  bool
	rtsp     *rtspv2.RTSPClient
	closed   bool
	closeCh  chan bool
	codecs   []av.CodecData
	viewers  map[string]bool
}

func IsCameraStreaming(cameraId string) bool {
	clientsMutex.RLock()
	defer clientsMutex.RUnlock()
	if client, ok := clients[cameraId]; ok && client != nil {
		return client.rtsp != nil
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
			cameraId: camera.Id,
			url:      camera.RtspUrl,
			running:  false,
			closed:   false,
			closeCh:  make(chan bool, 1),
			viewers:  make(map[string]bool),
		}
		clientsMutex.Lock()
		clients[camera.Id] = client
		clientsMutex.Unlock()
	}
	if !client.running {
		client.running = true
		client.closed = false
		for len(client.closeCh) > 0 {
			<-client.closeCh
		}
		log.Printf("%s: Starting %s\n", camera.Id, camera.RtspUrl)
		go rtspWorkerLoop(camera.Id, camera.RtspUrl)
	}
}

func clientSendQuit(cameraId string) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	if client, ok := clients[cameraId]; ok && client != nil {
		client.closed = true
		client.closeCh <- true
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
		client.running = false
	}
}

func clientSetRTSPClient(cameraId string, rtsp_client *rtspv2.RTSPClient) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	if client, ok := clients[cameraId]; ok && client != nil {
		client.rtsp = rtsp_client
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
		return client.running
	} else {
		return false
	}
}

func clientIsClosed(cameraId string) bool {
	clientsMutex.RLock()
	defer clientsMutex.RUnlock()
	if client, ok := clients[cameraId]; ok && client != nil {
		return client.closed
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
		client.codecs = codecs
	}
}

func WaitForClient(cameraId string) {
	for {
		clientsMutex.RLock()
		client, ok := clients[cameraId]
		clientsMutex.RUnlock()
		if ok && client != nil && client.running {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func GetCurrentCodecs(cameraId string) []av.CodecData {
	clientsMutex.RLock()
	defer clientsMutex.RUnlock()
	if client, ok := clients[cameraId]; ok && client != nil && client.codecs != nil {
		return client.codecs
	}
	return nil
}

func GetCodecs(cameraId string) []av.CodecData {
	for i := 0; i < config.Config.RTSP.Connect.Timeout.Seconds*10; i++ {
		clientsMutex.RLock()
		client, ok := clients[cameraId]
		clientsMutex.RUnlock()
		if !ok || client == nil {
			log.Printf("%s: No client\n", client.url)
			return nil
		}
		if client.codecs != nil {
			valid_codecs := len(client.codecs) > 0
			for _, codec := range client.codecs {
				if codec.Type() == av.H264 {
					codecVideo, ok := codec.(h264parser.CodecData)
					if !ok || codecVideo.SPS() == nil || codecVideo.PPS() == nil || len(codecVideo.SPS()) <= 0 || len(codecVideo.PPS()) <= 0 {
						log.Printf("%s: Bad Video Codec SPS or PPS Wait\n", client.url)
						valid_codecs = false
						break
					}
				}
			}
			if valid_codecs {
				log.Printf("%s: Ok Video Ready to play\n", client.url)
				return client.codecs
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

var viewersMutex sync.RWMutex
var viewers = make(map[string]*ViewerST)

const viewerChanSize = 1024

func setPacketTime(packet *av.Packet) *av.Packet {
	packet.Time = time.Duration(time.Now().UTC().UnixNano())
	return packet
}

func GetPacketTime(packet *av.Packet) time.Time {
	return time.UnixMicro(packet.Time.Microseconds()).UTC()
}

type ViewerST struct {
	Uuid   uuid.UUID
	Socket chan *av.Packet
}

func AddViewer(cameraId string) *ViewerST {
	clientsMutex.RLock()
	client, ok := clients[cameraId]
	clientsMutex.RUnlock()
	if ok && client != nil {
		viewer := ViewerST{
			Uuid:   uuid.New(),
			Socket: make(chan *av.Packet, viewerChanSize),
		}
		uuid := viewer.Uuid.String()

		clientsMutex.Lock()
		client.viewers[uuid] = true
		clientsMutex.Unlock()

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
		delete(client.viewers, uuidString)
		viewersMutex.Lock()
		delete(viewers, uuidString)
		viewersMutex.Unlock()
		log.Printf("%s: Closed camera viewer %s", cameraId, uuidString)
	}
}

func (v *ViewerST) cast(packet *av.Packet) {
	if len(v.Socket) < cap(v.Socket) {
		v.Socket <- packet
	}
}

func cast(cameraId string, packet *av.Packet) {
	clientsMutex.RLock()
	defer clientsMutex.RUnlock()
	if client, ok := clients[cameraId]; ok && client != nil {
		for id := range client.viewers {
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

func rtspWorkerLoop(cameraId, url string) {
	defer clientStop(cameraId)
	wait_s := time.Duration(1) * time.Second
	for {
		closed, err := rtspWorker(cameraId, url)
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

func rtspWorker(cameraId, url string) (bool, error) {
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
	quitCh := &client.closeCh
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
			cast(cameraId, setPacketTime(packetAV))
		}
	}
}

func runClients(subscriber *pubsub.Subscriber[services.CameraEvent]) {
	defer subscriber.Close()

	for e := range subscriber.C {
		switch (*e).Type() {
		case services.CameraAdded:
			event := (*e).(*services.AddCameraEvent)
			runIfNotRunning(event.Camera)
		case services.CameraUpdated:
			event := (*e).(*services.UpdateCameraEvent)
			if event.Camera.Disabled {
				clientDelete(event.Camera.Id)
			} else if event.PrevCamera != nil {
				clientSwap(event.Camera, event.PrevCamera)
			} else {
				runIfNotRunning(event.Camera)
			}
		case services.CameraDeleted:
			event := (*e).(*services.DeleteCameraEvent)
			clientDelete(event.Camera.Id)
		}
	}
}

func InitClients() {
	subscriber := services.CameraEventPubSub.Subscribe()
	go runClients(subscriber)
}
