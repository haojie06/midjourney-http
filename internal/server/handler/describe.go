package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/haojie06/midjourney-http/internal/discordmd"
)

func CreateDescribeTask(c *gin.Context) {
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}

	taskId := uuid.New().String()
	describeResultChan, err := discordmd.MidJourneyServiceApp.Describe(taskId, file, file.Filename, int(file.Size))
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	select {
	case result := <-describeResultChan:
		if result.Successful {
			c.JSON(200, gin.H{
				"message": result.Description,
			})
		} else {
			c.JSON(400, gin.H{"message": result.Message})
		}

	case <-time.After(5 * time.Minute):
		c.JSON(408, gin.H{"message": "timeout"})
	}
}
