package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/haojie06/midjourney-http/internal/discordmd"
	"github.com/haojie06/midjourney-http/internal/logger"
	"github.com/haojie06/midjourney-http/internal/model"
	"github.com/haojie06/midjourney-http/internal/utils"
)

func CreateGenerationTask(c *gin.Context) {
	var req model.GenerationTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.GinFailedWithMessage(c, 400, err.Error())
		return
	}
	taskId, taskResultChan, err := discordmd.MidJourneyServiceApp.Imagine(req.Prompt, req.Params, req.FastMode, req.AutoUpscale)
	if err != nil {
		if err == discordmd.ErrTooManyTasks {
			utils.GinFailedWithMessageAndTaskId(c, 429, taskId, err.Error())
		} else {
			utils.GinFailedWithMessageAndTaskId(c, 400, taskId, err.Error())
		}
		return
	}
	logger.Infof("task %s is created", taskId)
	// waiting for task to complete, block or webhook
	if req.ReportType == "webhook" {
		c.JSON(200, model.TaskHTTPResponse{
			TaskId: taskId,
			Status: "pending",
		})
		return
	}
	select {
	case <-time.After(60 * time.Minute):
		logger.Infof("task %s timeout", taskId)
		utils.GinFailedWithMessageAndTaskId(c, 408, taskId, "timeout")
		return
	case result := <-taskResultChan:
		// TODO implement webhook
		if !result.Successful {
			utils.GinFailedWithMessageAndTaskId(c, 400, taskId, result.Message)
			return
		}
		payload, ok := result.Payload.(discordmd.ImageGenerationResultPayload)
		if !ok {
			utils.GinFailedWithMessageAndTaskId(c, 400, taskId, "payload type error")
			return
		}
		logger.Infof("task %s completed", result.TaskId)
		c.JSON(200, model.TaskHTTPResponse{
			TaskId: result.TaskId,
			Status: "completed",
			Payload: model.GenerationTaskResponsePayload{
				OriginImageURL: payload.OriginImageURL,
				ImageURLs:      payload.ImageURLs,
			},
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
			utils.GinFailedWithMessageAndTaskId(c, 429, taskId, err.Error())
		} else {
			utils.GinFailedWithMessageAndTaskId(c, 400, taskId, err.Error())
		}
		return
	}
	select {
	case <-time.After(60 * time.Minute):
		logger.Infof("task %s timeout", taskId)
		utils.GinFailedWithMessageAndTaskId(c, 408, taskId, "timeout")
	case taskResult := <-taskResultChan:
		if !taskResult.Successful {
			utils.GinFailedWithMessageAndTaskId(c, 400, taskId, taskResult.Message)
			return
		}
		payload, ok := taskResult.Payload.(discordmd.ImageGenerationResultPayload)
		if !ok {
			utils.GinFailedWithMessageAndTaskId(c, 400, taskId, "failed to get payload")
			return
		}
		logger.Infof("task %s completed, success: %t", taskId, taskResult.Successful)
		c.JSON(200, model.TaskHTTPResponse{
			TaskId:  taskId,
			Status:  "completed",
			Message: taskResult.Message,
			Payload: model.GenerationTaskResponsePayload{
				ImageURLs:      payload.ImageURLs,
				OriginImageURL: payload.OriginImageURL,
			},
		})
	}

}
