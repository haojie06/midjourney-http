package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/haojie06/midjourney-http/internal/discordmd"
	"github.com/haojie06/midjourney-http/internal/logger"
	"github.com/haojie06/midjourney-http/internal/model"
)

func CreateGenerationTask(c *gin.Context) {
	var req model.GenerationTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	taskId, taskResultChan, err := discordmd.MidJourneyServiceApp.Imagine(req.Prompt, req.Params, req.FastMode, req.AutoUpscale)
	if err != nil {
		if err == discordmd.ErrTooManyTasks {
			c.JSON(429, gin.H{"message": err.Error()})
		} else {
			c.JSON(400, gin.H{"message": err.Error()})
		}
		return
	}
	logger.Infof("task %s is created", taskId)
	// waiting for task to complete, block or webhook
	if req.ReportType == "webhook" {
		c.JSON(200, model.GenerationTaskResponse{
			TaskId: taskId,
			Status: "pending",
		})
		return
	}
	select {
	case <-time.After(60 * time.Minute):
		logger.Infof("task %s timeout", taskId)
		c.JSON(408, gin.H{"message": "timeout"})
	case result := <-taskResultChan:
		// TODO implement webhook
		if !result.Successful {
			c.JSON(400, gin.H{"message": result.Message})
			return
		}
		payload, ok := result.Payload.(discordmd.ImageGenerationResultPayload)
		if !ok {
			c.JSON(400, gin.H{"message": "failed to get payload"})
			return
		}
		logger.Infof("task %s completed", result.TaskId)
		c.JSON(200, model.GenerationTaskResponse{
			TaskId:         result.TaskId,
			Status:         "completed",
			Message:        "success",
			OriginImageURL: payload.OriginImageURL,
			ImageURLs:      payload.ImageURLs,
		})
	}
}

func GenerationImageFromGetRequest(c *gin.Context) {
	prompt := c.Query("prompt")
	params := c.Query("params")
	fastMode := c.Query("fast") == "true"
	autoUpscale := c.Query("auto_upscale") == "true"
	taskId, taskResultChan, err := discordmd.MidJourneyServiceApp.Imagine(prompt, params, fastMode, autoUpscale)
	if err != nil {
		logger.Errorf("task %s failed: %s", taskId, err.Error())
		if err == discordmd.ErrTooManyTasks {
			c.JSON(429, gin.H{"message": err.Error()})
		} else {
			c.JSON(400, gin.H{"message": err.Error()})
		}
		return
	}
	select {
	case <-time.After(60 * time.Minute):
		logger.Infof("task %s timeout", taskId)
		c.JSON(408, gin.H{"message": "timeout"})
	case taskResult := <-taskResultChan:
		if !taskResult.Successful {
			c.JSON(400, gin.H{"message": taskResult.Message})
			return
		}
		payload, ok := taskResult.Payload.(discordmd.ImageGenerationResultPayload)
		if !ok {
			c.JSON(400, gin.H{"message": "failed to get payload"})
			return
		}
		logger.Infof("task %s completed, success: %t", taskId, taskResult.Successful)
		c.JSON(200, model.GenerationTaskResponse{
			TaskId:         taskId,
			Status:         "completed",
			Message:        taskResult.Message,
			ImageURLs:      payload.ImageURLs,
			OriginImageURL: payload.OriginImageURL,
		})
	}

}
