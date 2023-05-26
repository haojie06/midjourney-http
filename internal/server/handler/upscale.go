package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/haojie06/midjourney-http/internal/discordmd"
	"github.com/haojie06/midjourney-http/internal/logger"
	"github.com/haojie06/midjourney-http/internal/model"
	"github.com/haojie06/midjourney-http/internal/utils"
)

func CreateUpscaleTask(c *gin.Context) {
	var req model.UpscaleTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.GinFailedWithMessage(c, 400, err.Error())
		return
	}
	taskResultChan, err := discordmd.MidJourneyServiceApp.Upscale(req.TaskId, req.Index)
	if err != nil {
		utils.GinFailedWithMessageAndTaskId(c, 400, req.TaskId, err.Error())
		return
	}
	select {
	case <-time.After(30 * time.Minute):
		logger.Warnf("task %s timeout", req.TaskId)
		utils.GinFailedWithMessageAndTaskId(c, 408, req.TaskId, "timeout")
		return
	case taskResult := <-taskResultChan:
		if !taskResult.Successful {
			utils.GinFailedWithMessageAndTaskId(c, 400, req.TaskId, taskResult.Message)
			return
		}
		payload, ok := taskResult.Payload.(discordmd.ImageUpscaleResultPayload)
		if !ok {
			utils.GinFailedWithMessageAndTaskId(c, 400, req.TaskId, "payload type error")
			return
		}
		logger.Infof("task %s upscale %s completed", req.TaskId, payload.Index)
		c.JSON(200, model.TaskHTTPResponse{
			TaskId: req.TaskId,
			Status: "completed",
			Payload: model.UpscaleTaskResponsePayload{
				ImageURL: payload.ImageURL,
				Index:    payload.Index,
			},
		})
		return
	}
}

func UpscaleImageFromGetRequest(c *gin.Context) {
	taskId := c.Query("task_id")
	upscaleIndex := c.Query("index")
	resultChan, err := discordmd.MidJourneyServiceApp.Upscale(taskId, upscaleIndex)
	if err != nil {
		utils.GinFailedWithMessageAndTaskId(c, 400, taskId, err.Error())
		return
	}
	select {
	case <-time.After(10 * time.Minute):
		// discordmd.MidJourneyServiceApp.RemoveTaskRuntime(taskId)
		logger.Warnf("task %s timeout", taskId)
		utils.GinFailedWithMessageAndTaskId(c, 408, taskId, "timeout")
		return
	case taskResult := <-resultChan:
		if !taskResult.Successful {
			c.JSON(400, model.TaskHTTPResponse{
				Status:  "failed",
				Message: taskResult.Message,
			})
			return
		}
		payload, ok := taskResult.Payload.(discordmd.ImageUpscaleResultPayload)
		if !ok {
			utils.GinFailedWithMessageAndTaskId(c, 400, taskId, "payload type error")
			return
		}
		logger.Infof("task %s upscale %s completed", taskId, payload.Index)
		c.JSON(200, model.TaskHTTPResponse{
			TaskId: taskId,
			Status: "completed",
			Payload: model.UpscaleTaskResponsePayload{
				ImageURL: payload.ImageURL,
				Index:    payload.Index,
			},
		})
		return
	}
}
