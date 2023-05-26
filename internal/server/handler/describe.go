package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/haojie06/midjourney-http/internal/discordmd"
	"github.com/haojie06/midjourney-http/internal/model"
	"github.com/haojie06/midjourney-http/internal/utils"
)

func CreateDescribeTask(c *gin.Context) {
	file, err := c.FormFile("image")
	if err != nil {
		utils.GinFailedWithMessage(c, 400, err.Error())
		return
	}

	taskId, resultChan, err := discordmd.MidJourneyServiceApp.Describe(file, file.Filename, int(file.Size))
	if err != nil {
		utils.GinFailedWithMessageAndTaskId(c, 400, taskId, err.Error())
		return
	}
	select {
	case result := <-resultChan:
		if !result.Successful {
			utils.GinFailedWithMessageAndTaskId(c, 400, taskId, result.Message)
			return
		}
		payload, ok := result.Payload.(discordmd.ImageDescribeResultPayload)
		if !ok {
			utils.GinFailedWithMessageAndTaskId(c, 400, taskId, "failed to get payload")
			return
		}
		c.JSON(200, model.TaskHTTPResponse{
			TaskId: taskId,
			Status: "completed",
			Payload: model.DescribeTaskResponsePayload{
				Description: payload.Description,
			},
		})
		return
	case <-time.After(5 * time.Minute):
		utils.GinFailedWithMessageAndTaskId(c, 408, taskId, "timeout")
		return
	}
}
