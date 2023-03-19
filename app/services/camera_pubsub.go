package services

import (
	"github.com/aicacia/pubsub"
	"github.com/aicacia/streams/app/models"
)

const (
	CameraAdded = 1 + iota
	CameraUpdated
	CameraDeleted
)

type CameraEvent interface {
	Type() int
}

type AddCameraEvent struct {
	Camera *models.CameraST
}

func (e *AddCameraEvent) Type() int {
	return CameraAdded
}

type UpdateCameraEvent struct {
	Camera     *models.CameraST
	PrevCamera *models.CameraST
}

func (e *UpdateCameraEvent) Type() int {
	return CameraUpdated
}

type DeleteCameraEvent struct {
	Camera *models.CameraST
}

func (e *DeleteCameraEvent) Type() int {
	return CameraDeleted
}

var CameraEventPubSub = pubsub.NewPubSub[CameraEvent]()

func onAddCamera(camera *models.CameraST) {
	var event CameraEvent = &AddCameraEvent{
		Camera: camera,
	}
	CameraEventPubSub.Publish(&event)
}

func onUpdateCamera(camera, prevCamera *models.CameraST) {
	var event CameraEvent = &UpdateCameraEvent{
		Camera:     camera,
		PrevCamera: prevCamera,
	}
	CameraEventPubSub.Publish(&event)
}

func onRemoveCamera(camera *models.CameraST) {
	var event CameraEvent = &DeleteCameraEvent{
		Camera: camera,
	}
	CameraEventPubSub.Publish(&event)
}
