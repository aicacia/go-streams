package controllers

import (
	"log"
	"net/http"

	"github.com/aicacia/streams/app/config"
	"github.com/aicacia/streams/app/models"
	"github.com/aicacia/streams/app/rtsp"
	"github.com/aicacia/streams/app/util"
	webrtc "github.com/deepch/vdk/format/webrtcv3"
	"github.com/gofiber/fiber/v2"
)

// Auth GetLiveCodecs
//
//		@Summary		Get Live Codecs
//		@Description	get camera live codecs
//		@Tags			cameras,live
//		@Accept			json
//		@Produce		json
//	    @Param			cameraId	path		string	true	"Camera ID"
//		@Success		200	{object}	[]string
//		@Failure		400	{object}	models.ResponseErrorST
//		@Failure		401	{object}	models.ResponseErrorST
//		@Failure		404	{object}	models.ResponseErrorST
//		@Failure		500	{object}	models.ResponseErrorST
//		@Router			/cameras/{cameraId}/live/codecs [get]
func GetLiveCodecs(c *fiber.Ctx) error {
	cameraId := c.Params("cameraId")
	codecs := rtsp.GetCodecs(cameraId)
	if codecs == nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(models.ResponseErrorST{
			Error: "Failed to start local rtsp",
		})
	}
	c.Status(http.StatusOK)
	return c.JSON(util.CodecsToStrings(codecs))
}

// Auth PostLiveSdp
//
//		@Summary		Send live offer
//		@Description	send live offer for camera by id
//		@Tags			cameras,live
//		@Accept			json
//		@Produce		json
//	    @Param			cameraId	path		string	true	"Camera ID"
//	    @Param			offer	body    models.OfferBodyST	true	"Offer body"
//		@Success		200	{object}	models.AnswerST
//		@Failure		400	{object}	models.ResponseErrorST
//		@Failure		401	{object}	models.ResponseErrorST
//		@Failure		404	{object}	models.ResponseErrorST
//		@Failure		500	{object}	models.ResponseErrorST
//		@Router			/cameras/{cameraId}/live/sdp [post]
func PostLiveSdp(c *fiber.Ctx) error {
	cameraId := c.Params("cameraId")
	var body models.OfferBodyST
	if err := c.BodyParser(&body); err != nil {
		log.Println(err)
		c.Status(http.StatusBadRequest)
		return c.JSON(models.ResponseErrorST{
			Error: "Invalid Request Body",
		})
	}
	codecs := rtsp.GetCodecs(cameraId)
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
	viewer := rtsp.AddViewer(cameraId)
	if viewer == nil {
		log.Printf("Failed to create viewer for %s\n", cameraId)
		c.Status(http.StatusInternalServerError)
		return c.JSON(models.ResponseErrorST{
			Error: "Failed to Create viewer for stream",
		})
	}

	go func() {
		defer rtsp.DeleteViewer(cameraId, &viewer.Uuid)
		defer muxerWebRTC.Close()

		for packet := range viewer.Socket {
			err = muxerWebRTC.WritePacket(*packet)
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
