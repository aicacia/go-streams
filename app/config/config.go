package config

import (
	"io"
	"log"
	"os"
	"strings"

	"github.com/magiconair/properties"
)

var Config ConfigST
var configPath string

type ConfigST struct {
	Host string `json:"host" properties:"host,default="`
	Port int    `json:"port" properties:"port,default=9090"`
	Log  struct {
		File string `json:"file" properties:"file,default=ui.log"`
	} `json:"log" properties:"log"`
	Cameras struct {
		Folder string `json:"folders" properties:"folders,default=cameras"`
	} `json:"cameras" properties:"cameras"`
	Recordings struct {
		Folder string `json:"folders" properties:"folders,default=recordings"`
	} `json:"recordings" properties:"recordings"`
	Ice struct {
		Servers    []string `json:"servers" properties:"servers,default="`
		Username   string   `json:"username" properties:"username,default="`
		Credential string   `json:"credential" properties:"credential,default="`
	} `json:"ice" properties:"ice"`
	RTSP struct {
		Connect struct {
			Timeout struct {
				Seconds int `json:"seconds" properties:"seconds,default=10"`
			} `json:"timeout" properties:"timeout"`
		} `json:"connect" properties:"connect"`
		IO struct {
			Timeout struct {
				Seconds int `json:"seconds" properties:"seconds,default=10"`
			} `json:"timeout" properties:"timeout"`
		} `json:"io" properties:"io"`
		Playback struct {
			CodecDelayMS int `json:"codec_delay_ms" properties:"codec_delay_ms,default=3000"`
		} `json:"playback" properties:"playback"`
		Debug bool `json:"debug" properties:"debug,default=false"`
	} `json:"rtsp" properties:"rtsp"`
}

func loadConfig(configPath string) error {
	log.Printf("Loading Config from %s\n", configPath)
	file, err := os.Open(configPath)
	if err != nil {
		return err
	}
	defer file.Close()
	bytes, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	log.Printf("Config:\n%s\n\n", string(bytes))
	p, err := properties.Load(bytes, properties.UTF8)
	if err != nil {
		return err
	}
	decode_err := p.Decode(&Config)
	if decode_err != nil {
		return decode_err
	}
	return nil
}

func RefreshConfig() error {
	return loadConfig(configPath)
}

func InitConfig() {
	argConfigJSON := os.Args[len(os.Args)-1]
	path := "/app/config.properties"
	if strings.HasSuffix(argConfigJSON, ".properties") {
		path = argConfigJSON
	}
	configPath = path
	err := loadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}
}
