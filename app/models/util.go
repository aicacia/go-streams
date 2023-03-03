package models

type ResponseErrorST struct {
	Error string `json:"error"validate:"required"`
}

type HealthCheckST struct {
	Ok bool `json:"ok"validate:"required"`
}

type VersionST struct {
	Version string `json:"version"validate:"required"`
	Build   string `json:"build"validate:"required"`
}
