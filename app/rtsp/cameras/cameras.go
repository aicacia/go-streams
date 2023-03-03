package cameras

import (
	"sync"

	"github.com/aicacia/streams/app/models"
	"github.com/aicacia/streams/pubsub"
)

var camerasMutex sync.RWMutex
var cameras = make(map[string]*models.CameraST)

var EventPubSub = pubsub.NewPubSub[CameraEventST]()

const (
	Added = 1 + iota
	Updated
	Deleted
)

type CameraEventST struct {
	Type       int
	Camera     *models.CameraST
	PrevCamera *models.CameraST
}

func AddCamera(camera *models.CameraST) {
	camerasMutex.Lock()
	cameras[camera.Id] = camera
	camerasMutex.Unlock()

	EventPubSub.Publish(&CameraEventST{
		Type:   Added,
		Camera: camera,
	})
}

func UpdateCamera(camera *models.CameraST) {
	var prev_camera *models.CameraST
	if current_camera, ok := cameras[camera.Id]; ok && current_camera != nil {
		prev_camera = current_camera
	}
	camerasMutex.Lock()
	cameras[camera.Id] = camera
	camerasMutex.Unlock()

	EventPubSub.Publish(&CameraEventST{
		Type:       Updated,
		Camera:     camera,
		PrevCamera: prev_camera,
	})
}

func RemoveCamera(camera *models.CameraST) {
	camerasMutex.Lock()
	delete(cameras, camera.Id)
	camerasMutex.Unlock()

	EventPubSub.Publish(&CameraEventST{
		Type:   Deleted,
		Camera: camera,
	})
}
