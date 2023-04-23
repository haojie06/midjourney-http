package handler

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/haojie06/midjourney-http/internal/discordmd"
	"github.com/haojie06/midjourney-http/internal/model"
)

func CreateGenerationTask(c *gin.Context) {
	var req model.GenerationTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	taskId, taskResultChan, err := discordmd.MidJourneyServiceApp.Imagine(req.Prompt, req.Params)
	if err != nil {
		if err == discordmd.ErrTooManyTasks {
			c.JSON(429, gin.H{"message": err.Error()})
		} else {
			c.JSON(400, gin.H{"message": err.Error()})
		}
		return
	}
	log.Printf("task %s created\n", taskId)
	// waiting for task to complete, block or webhook
	if req.ReportType == "webhook" {
		c.JSON(200, model.GenerationTaskResponse{
			TaskId: taskId,
			Status: "pending",
		})
		return
	}
	taskResult := <-taskResultChan
	c.JSON(200, model.GenerationTaskResponse{
		TaskId:         taskId,
		Status:         "completed",
		ImageURLs:      taskResult.ImageURLs,
		OriginImageURL: taskResult.OriginImageURL,
	})
}

func GenerationImageFromGetRequest(c *gin.Context) {
	prompt := c.Query("prompt")
	params := c.Query("params")
	taskId, taskResultChan, err := discordmd.MidJourneyServiceApp.Imagine(prompt, params)
	if err != nil {
		if err == discordmd.ErrTooManyTasks {
			c.JSON(429, gin.H{"message": err.Error()})
		} else {
			c.JSON(400, gin.H{"message": err.Error()})
		}
		return
	}
	log.Printf("task %s created\n", taskId)
	taskResult := <-taskResultChan
	c.JSON(200, model.GenerationTaskResponse{
		TaskId:         taskId,
		Status:         "completed",
		ImageURLs:      taskResult.ImageURLs,
		OriginImageURL: taskResult.OriginImageURL,
	})
}
