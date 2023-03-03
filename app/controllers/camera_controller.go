package controllers

import (
	"log"
	"net/http"

	"github.com/aicacia/streams/app/models"
	"github.com/aicacia/streams/app/services"
	"github.com/gofiber/fiber/v2"
)

// Auth GetCameras
//
//	@Summary		Get Cameras
//	@Description	list all cameras
//	@Tags			cameras
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	    []models.CameraST
//	@Failure		400	{object}	models.ResponseErrorST
//	@Failure		401	{object}	models.ResponseErrorST
//	@Failure		404	{object}	models.ResponseErrorST
//	@Failure		500	{object}	models.ResponseErrorST
//	@Router			/cameras [get]
func GetCameras(c *fiber.Ctx) error {
	cameras, err := services.ListCameras()
	if err != nil {
		log.Println(err)
		if len(cameras) == 0 {
			c.Status(http.StatusInternalServerError)
			return c.JSON(models.ResponseErrorST{
				Error: err.Error(),
			})
		}
	}
	c.Status(http.StatusOK)
	return c.JSON(cameras)
}

// Auth GetCameraById
//
//		@Summary		Get Camera by id
//		@Description	get camera by id
//		@Tags			cameras
//		@Accept			json
//		@Produce		json
//	    @Param			cameraId	path		string	true	"Camera ID"
//		@Success		200	{object}	models.CameraST
//		@Failure		400	{object}	models.ResponseErrorST
//		@Failure		401	{object}	models.ResponseErrorST
//		@Failure		404	{object}	models.ResponseErrorST
//		@Failure		500	{object}	models.ResponseErrorST
//		@Router			/cameras/{cameraId} [get]
func GetCameraById(c *fiber.Ctx) error {
	camera, err := services.GetCamera(c.Params("cameraId"))
	if err != nil {
		log.Println(err)
		c.Status(http.StatusBadRequest)
		return c.JSON(models.ResponseErrorST{
			Error: err.Error(),
		})
	}
	c.Status(http.StatusOK)
	return c.JSON(camera)
}

// Auth PostCreateCamera
//
//		@Summary		Create Camera
//		@Description	create a camera
//		@Tags			cameras
//		@Accept			json
//		@Produce		json
//	    @Param			camera	body    services.CameraCreateST	true	"Create Camera"
//		@Success		200	{object}	models.CameraST
//		@Failure		400	{object}	models.ResponseErrorST
//		@Failure		401	{object}	models.ResponseErrorST
//		@Failure		404	{object}	models.ResponseErrorST
//		@Failure		500	{object}	models.ResponseErrorST
//		@Router			/cameras [post]
func PostCreateCamera(c *fiber.Ctx) error {
	var body services.CameraCreateST
	if err := c.BodyParser(&body); err != nil {
		log.Println(err)
		c.Status(http.StatusBadRequest)
		return c.JSON(models.ResponseErrorST{
			Error: err.Error(),
		})
	}
	camera, err := services.CreateCamera(&body)
	if err != nil {
		log.Println(err)
		c.Status(http.StatusBadRequest)
		return c.JSON(models.ResponseErrorST{
			Error: err.Error(),
		})
	}
	c.Status(http.StatusCreated)
	return c.JSON(camera)
}

// Auth PatchUpdateCamera
//
//		@Summary		Update Camera
//		@Description	update a camera by id
//		@Tags			cameras
//		@Accept			json
//		@Produce		json
//	    @Param			cameraId	path		string	true	"Camera ID"
//	    @Param			camera	body    services.CameraUpdateST	true	"Update Camera"
//		@Success		200	{object}	models.CameraST
//		@Failure		400	{object}	models.ResponseErrorST
//		@Failure		401	{object}	models.ResponseErrorST
//		@Failure		404	{object}	models.ResponseErrorST
//		@Failure		500	{object}	models.ResponseErrorST
//		@Router			/cameras/{cameraId} [patch]
func PatchUpdateCamera(c *fiber.Ctx) error {
	var body services.CameraUpdateST
	if err := c.BodyParser(&body); err != nil {
		log.Println(err)
		c.Status(http.StatusBadRequest)
		return c.JSON(models.ResponseErrorST{
			Error: err.Error(),
		})
	}
	camera, err := services.UpdateCamera(c.Params("cameraId"), &body)
	if err != nil {
		log.Println(err)
		c.Status(http.StatusBadRequest)
		return c.JSON(models.ResponseErrorST{
			Error: err.Error(),
		})
	}
	c.Status(http.StatusOK)
	return c.JSON(camera)
}

// Auth DeleteCamera
//
//	@Summary		Delete Camera
//	@Description	delette camera by id
//	@Tags			cameras
//	@Accept			json
//	@Produce		json
//	@Param			cameraId	path		string	true	"Camera ID"
//	@Success		204	{null}	    nil
//	@Failure		400	{object}	models.ResponseErrorST
//	@Failure		401	{object}	models.ResponseErrorST
//	@Failure		404	{object}	models.ResponseErrorST
//	@Failure		500	{object}	models.ResponseErrorST
//	@Router			/cameras/{cameraId} [delete]
func DeleteCamera(c *fiber.Ctx) error {
	disabled := true
	_, err := services.UpdateCamera(c.Params("cameraId"), &services.CameraUpdateST{
		Disabled: &disabled,
	})
	if err != nil {
		log.Println(err)
		c.Status(http.StatusBadRequest)
		return c.JSON(models.ResponseErrorST{
			Error: err.Error(),
		})
	}
	c.Status(http.StatusNoContent)
	return c.Send(nil)
}
