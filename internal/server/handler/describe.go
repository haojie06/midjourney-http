package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/haojie06/midjourney-http/internal/discordmd"
)

func CreateDescribeTask(c *gin.Context) {
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}

	_, resultChan, err := discordmd.MidJourneyServiceApp.Describe(file, file.Filename, int(file.Size))
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	select {
	case result := <-resultChan:
		if !result.Successful {
			c.JSON(400, gin.H{"message": result.Message})
			return
		}
		payload, ok := result.Payload.(discordmd.ImageDescribeResultPayload)
		if !ok {
			c.JSON(400, gin.H{"message": "failed to get payload"})
			return
		}
		c.JSON(200, gin.H{
			"message": payload.Description,
		})

	case <-time.After(5 * time.Minute):
		c.JSON(408, gin.H{"message": "timeout"})
	}
}
