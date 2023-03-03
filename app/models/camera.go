package models

import "time"

type CameraST struct {
	Id        string    `json:"id"validate:"required"`
	Name      string    `json:"name"validate:"required"`
	Url       string    `json:"url"validate:"required"`
	RtspUrl   string    `json:"rtsp_url"validate:"required"`
	Disabled  bool      `json:"disabled"validate:"required"`
	Recording bool      `json:"recording"validate:"required"`
	CreatedTs time.Time `json:"created_ts"validate:"required"`
	UpdatedTs time.Time `json:"updated_ts"validate:"required"`
}
