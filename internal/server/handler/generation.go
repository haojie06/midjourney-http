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
		// TODO
		// discordmd.MidJourneyServiceApp.RemoveTaskRuntime(taskId)
		logger.Infof("task %s timeout", taskId)
		c.JSON(408, gin.H{"message": "timeout"})
	case taskResult := <-taskResultChan:
		// TODO implement webhook
		logger.Infof("task %s completed", taskResult.TaskId)
		c.JSON(200, model.GenerationTaskResponse{
			TaskId:         taskResult.TaskId,
			Status:         "completed",
			Message:        "success",
			OriginImageURL: taskResult.OriginImageURL,
			ImageURLs:      taskResult.ImageURLs,
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
		logger.Infof("task %s completed, success: %t", taskId, taskResult.Successful)
		if taskResult.Successful {
			c.JSON(200, model.GenerationTaskResponse{
				TaskId:         taskId,
				Status:         "completed",
				Message:        taskResult.Message,
				ImageURLs:      taskResult.ImageURLs,
				OriginImageURL: taskResult.OriginImageURL,
			})
		} else {
			c.JSON(400, model.GenerationTaskResponse{
				TaskId:         taskId,
				Status:         "failed",
				Message:        taskResult.Message,
				ImageURLs:      taskResult.ImageURLs,
				OriginImageURL: taskResult.OriginImageURL,
			})
		}
	}

}
