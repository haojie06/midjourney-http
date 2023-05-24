package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/haojie06/midjourney-http/internal/discordmd"
)

func CreateDescribeTask(c *gin.Context) {
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	fileReader, err := file.Open()
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	defer fileReader.Close()
	desc, err := discordmd.MidJourneyServiceApp.Describe(fileReader, file.Filename, int(file.Size))
	if err != nil {
		c.JSON(400, gin.H{"message": desc})
		return
	}
}
