package controllers

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/aicacia/streams/app/config"
	"github.com/aicacia/streams/app/models"
	"github.com/aicacia/streams/app/rtsp"
	"github.com/aicacia/streams/app/util"
	webrtc "github.com/deepch/vdk/format/webrtcv3"
	"github.com/gofiber/fiber/v2"
)

// Auth CreatePlayback
//
//		@Summary		Create Playback
//		@Description	create a new camera playback
//		@Tags			cameras,playback
//		@Accept			json
//		@Produce		json
//	    @Param			cameraId	path		string	true	"Camera ID"
//	    @Param			start	    query		int64	true	"Playback start time"
//		@Success		200	{object}	string
//		@Failure		400	{object}	models.ResponseErrorST
//		@Failure		401	{object}	models.ResponseErrorST
//		@Failure		404	{object}	models.ResponseErrorST
//		@Failure		500	{object}	models.ResponseErrorST
//		@Router			/cameras/{cameraId}/playback [post]
func PostCreatePlayback(c *fiber.Ctx) error {
	cameraId := c.Params("cameraId")
	startString := c.Query("start")
	var start *time.Time
	if len(startString) != 0 {
		timestampMS, err := strconv.ParseInt(startString, 10, 64)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return c.JSON(models.ResponseErrorST{
				Error: "Invalid start time",
			})
		}
		t := time.UnixMilli(timestampMS).UTC()
		start = &t
	}
	playbackId, err := rtsp.NewPlayback(cameraId, start)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return c.JSON(models.ResponseErrorST{
			Error: "Failed to create Playback",
		})
	}
	c.Status(http.StatusOK)
	return c.JSON(playbackId)
}

// Auth GetPlaybackCodecs
//
//		@Summary		Get Playbaack Codecs
//		@Description	get camera playback codecs
//		@Tags			cameras,playback
//		@Accept			json
//		@Produce		json
//	    @Param			playbackId	path		string	true	"Playback ID"
//		@Success		200	{object}	[]string
//		@Failure		400	{object}	models.ResponseErrorST
//		@Failure		401	{object}	models.ResponseErrorST
//		@Failure		404	{object}	models.ResponseErrorST
//		@Failure		500	{object}	models.ResponseErrorST
//		@Router			/playback/{playbackId}/codecs [get]
func GetPlaybackCodecs(c *fiber.Ctx) error {
	playbackId := c.Params("playbackId")
	codecs := rtsp.GetPlaybackCodecs(playbackId)
	if codecs == nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(models.ResponseErrorST{
			Error: "Failed to start local rtsp",
		})
	}
	c.Status(http.StatusOK)
	return c.JSON(util.CodecsToStrings(codecs))
}

// Auth PostPlaybackSdp
//
//		@Summary		Send playback offer
//		@Description	send playback offer for camera by id
//		@Tags			cameras,playback
//		@Accept			json
//		@Produce		json
//	    @Param			playbackId	path		string	true	"Playback ID"
//	    @Param			offer	body    models.OfferBodyST	true	"Offer body"
//		@Success		200	{object}	models.AnswerST
//		@Failure		400	{object}	models.ResponseErrorST
//		@Failure		401	{object}	models.ResponseErrorST
//		@Failure		404	{object}	models.ResponseErrorST
//		@Failure		500	{object}	models.ResponseErrorST
//		@Router			/playback/{playbackId}/sdp [post]
func PostPlaybackSdp(c *fiber.Ctx) error {
	playbackId := c.Params("playbackId")
	var body models.OfferBodyST
	if err := c.BodyParser(&body); err != nil {
		log.Println(err)
		c.Status(http.StatusBadRequest)
		return c.JSON(models.ResponseErrorST{
			Error: "Invalid Request Body",
		})
	}
	codecs := rtsp.GetPlaybackCodecs(playbackId)
	if codecs == nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(models.ResponseErrorST{
			Error: "Stream Codec Not Found",
		})
	}

	muxerWebRTC := webrtc.NewMuxer(webrtc.Options{
		ICEServers:    config.Config.Ice.Servers,
		ICEUsername:   config.Config.Ice.Username,
		ICECredential: config.Config.Ice.Credential,
		PortMin:       0,
		PortMax:       0,
	})
	answer, err := muxerWebRTC.WriteHeader(codecs, body.OfferBase64)
	if err != nil {
		log.Println("WriteHeader", err)
		c.Status(http.StatusInternalServerError)
		return c.JSON(models.ResponseErrorST{
			Error: "Failed to Start stream",
		})
	}
	socket := rtsp.GetPlaybackSocket(playbackId)
	if socket == nil {
		log.Printf("Failed to create viewer for %s\n", playbackId)
		c.Status(http.StatusInternalServerError)
		return c.JSON(models.ResponseErrorST{
			Error: "Failed to Create viewer for stream",
		})
	}

	go func() {
		defer rtsp.PlaybackDelete(playbackId)
		defer muxerWebRTC.Close()

		for packet := range socket {
			err = muxerWebRTC.WritePacket(packet)
			if err != nil {
				log.Println("WritePacket", err)
				return
			}
		}
	}()

	c.Status(http.StatusOK)
	return c.JSON(models.AnswerST{
		AnswerBase64: answer,
	})
}
