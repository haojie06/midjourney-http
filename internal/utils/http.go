package utils

import (
	"github.com/gin-gonic/gin"
	"github.com/haojie06/midjourney-http/internal/model"
)

func GinFailedWithMessage(c *gin.Context, status int, message string) {
	c.JSON(status, model.TaskHTTPResponse{
		Status:  "failed",
		Message: message,
	})
}

func GinFailedWithMessageAndTaskId(c *gin.Context, status int, taskId string, message string) {
	c.JSON(status, model.TaskHTTPResponse{
		TaskId:  taskId,
		Status:  "failed",
		Message: message,
	})
}
