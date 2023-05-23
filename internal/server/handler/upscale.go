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
	upscaleResultChan, err := discordmd.MidJourneyServiceApp.Upscale(req.TaskId, req.Index)
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	select {
	case <-time.After(60 * time.Minute):
		logger.Warnf("task %s timeout", req.TaskId)
	case taskResult := <-upscaleResultChan:
		logger.Infof("task %s upscale %s completed", taskResult.TaskId, taskResult.Index)
		c.JSON(200, model.UpscaleTaskResponse{
			TaskId:   taskResult.TaskId,
			Status:   "completed",
			Message:  "success",
			ImageURL: taskResult.ImageURL,
			Index:    taskResult.Index,
		})
	}
}

func UpscaleImageFromGetRequest(c *gin.Context) {
	taskId := c.Query("task_id")
	upscaleIndex := c.Query("index")
	upscaleResultChan, err := discordmd.MidJourneyServiceApp.Upscale(taskId, upscaleIndex)
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	select {
	case <-time.After(10 * time.Minute):
		// discordmd.MidJourneyServiceApp.RemoveTaskRuntime(taskId)
		logger.Warnf("task %s timeout", taskId)
		c.JSON(408, gin.H{"message": "timeout"})
	case taskResult := <-upscaleResultChan:
		logger.Infof("task %s upscale %s completed", taskResult.TaskId, taskResult.Index)
		c.JSON(200, model.UpscaleTaskResponse{
			TaskId:   taskResult.TaskId,
			Status:   "completed",
			Message:  "success",
			ImageURL: taskResult.ImageURL,
			Index:    taskResult.Index,
		})
	}
}
