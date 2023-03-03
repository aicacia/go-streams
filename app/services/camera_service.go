package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/aicacia/streams/app/config"
	"github.com/aicacia/streams/app/models"
	"github.com/aicacia/streams/app/rtsp/cameras"
	"github.com/google/uuid"
)

var camerasCreateMutex sync.Mutex
var camerasUpdateMutex sync.Mutex

func readCamera(path string) (*models.CameraST, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var camera models.CameraST
	json_err := json.Unmarshal(bytes, &camera)
	if json_err != nil {
		return nil, json_err
	}
	return &camera, nil
}

func writeCamera(path string, camera *models.CameraST) error {
	camerasUpdateMutex.Lock()
	defer camerasUpdateMutex.Unlock()
	bytes, err := json.Marshal(camera)
	if err != nil {
		return err
	}
	write_err := os.WriteFile(path, bytes, 0644)
	if write_err != nil {
		return write_err
	}
	return nil
}

func cameraExists(id string) error {
	_, err := os.Stat(cameraPath(id))
	if err != nil {
		return err
	}
	return nil
}

func cameraPath(id string) string {
	return path.Join(config.Config.Cameras.Folder, fmt.Sprintf("%s.json", id))
}

func GetCamera(id string) (*models.CameraST, error) {
	return readCamera(cameraPath(id))
}

func ListCameras() ([]*models.CameraST, error) {
	cameras := make([]*models.CameraST, 0)
	err := filepath.WalkDir(config.Config.Cameras.Folder, func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			camera, err := readCamera(path)
			if err != nil {
				return err
			}
			cameras = append(cameras, camera)
		}
		return nil
	})
	if err != nil {
		return cameras, err
	}
	return cameras, nil
}

type CameraCreateST struct {
	Name      string `json:"name"`
	Url       string `json:"url"`
	RtspUrl   string `json:"rtsp_url"`
	Disabled  bool   `json:"disabled"`
	Recording bool   `json:"recording"`
}

func CreateCamera(create_camera *CameraCreateST) (*models.CameraST, error) {
	camerasCreateMutex.Lock()
	defer camerasCreateMutex.Unlock()
	id, err := createCameraUUID(100)
	if err != nil {
		return nil, err
	}
	camera := models.CameraST{
		Id:        id,
		Name:      create_camera.Name,
		Url:       create_camera.Url,
		RtspUrl:   create_camera.RtspUrl,
		Disabled:  create_camera.Disabled,
		Recording: create_camera.Recording,
		CreatedTs: time.Now().UTC(),
		UpdatedTs: time.Now().UTC(),
	}
	write_err := writeCamera(cameraPath(id), &camera)
	if write_err != nil {
		return nil, write_err
	}
	cameras.AddCamera(&camera)
	return &camera, nil
}

type CameraUpdateST struct {
	Name      *string `json:"name"`
	Url       *string `json:"url"`
	RtspUrl   *string `json:"rtsp_url"`
	Disabled  *bool   `json:"disabled"`
	Recording *bool   `json:"recording"`
}

func UpdateCamera(id string, update_camera *CameraUpdateST) (*models.CameraST, error) {
	path := cameraPath(id)
	camera, err := readCamera(path)
	if err != nil {
		return nil, err
	}
	if update_camera.Name != nil {
		camera.Name = *update_camera.Name
	}
	if update_camera.Url != nil {
		camera.Url = *update_camera.Url
	}
	if update_camera.RtspUrl != nil {
		camera.RtspUrl = *update_camera.RtspUrl
	}
	if update_camera.Disabled != nil {
		camera.Disabled = *update_camera.Disabled
	}
	if update_camera.Recording != nil {
		camera.Recording = *update_camera.Recording
	}
	camera.UpdatedTs = time.Now().UTC()
	update_err := writeCamera(path, camera)
	if update_err != nil {
		return nil, update_err
	}
	cameras.UpdateCamera(camera)
	return camera, nil
}

func createCameraUUID(retries int) (string, error) {
	for {
		id := uuid.New().String()
		err := cameraExists(id)
		if err == nil {
			return id, nil
		} else if retries <= 0 {
			return "", errors.New("failed to create camera id")
		} else {
			retries--
		}
	}
}
