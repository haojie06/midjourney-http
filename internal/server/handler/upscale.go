package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/haojie06/midjourney-http/internal/discordmd"
	"github.com/haojie06/midjourney-http/internal/logger"
	"github.com/haojie06/midjourney-http/internal/model"
)

func CreateUpscaleTask(c *gin.Context) {
	var req model.UpscaleTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	taskResultChan, err := discordmd.MidJourneyServiceApp.Upscale(req.TaskId, req.Index)
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	select {
	case <-time.After(30 * time.Minute):
		logger.Warnf("task %s timeout", req.TaskId)
	case taskResult := <-taskResultChan:
		if !taskResult.Successful {
			c.JSON(400, gin.H{"message": taskResult.Message})
			return
		}
		payload, ok := taskResult.Payload.(discordmd.ImageUpscaleResultPayload)
		if !ok {
			c.JSON(400, gin.H{"message": "payload type error"})
			return
		}
		logger.Infof("task %s upscale %s completed", req.TaskId, payload.Index)
		c.JSON(200, model.UpscaleTaskResponse{
			TaskId:   req.TaskId,
			Status:   "completed",
			Message:  "success",
			ImageURL: payload.ImageURL,
			Index:    payload.Index,
		})
	}
}

func UpscaleImageFromGetRequest(c *gin.Context) {
	taskId := c.Query("task_id")
	upscaleIndex := c.Query("index")
	resultChan, err := discordmd.MidJourneyServiceApp.Upscale(taskId, upscaleIndex)
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	select {
	case <-time.After(10 * time.Minute):
		// discordmd.MidJourneyServiceApp.RemoveTaskRuntime(taskId)
		logger.Warnf("task %s timeout", taskId)
		c.JSON(408, gin.H{"message": "timeout"})
	case taskResult := <-resultChan:
		if !taskResult.Successful {
			c.JSON(400, gin.H{"message": taskResult.Message})
			return
		}
		payload, ok := taskResult.Payload.(discordmd.ImageUpscaleResultPayload)
		if !ok {
			c.JSON(400, gin.H{"message": "payload type error"})
			return
		}
		logger.Infof("task %s upscale %s completed", taskId, payload.Index)
		c.JSON(200, model.UpscaleTaskResponse{
			TaskId:   taskId,
			Status:   "completed",
			Message:  "success",
			ImageURL: payload.ImageURL,
			Index:    payload.Index,
		})
	}
}
