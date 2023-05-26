package server

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/pprof"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/haojie06/midjourney-http/internal/logger"
	"github.com/haojie06/midjourney-http/internal/server/handler"
)

func Start(host, port, apiKey string) {
	router := InnitRouter(apiKey)
	if err := router.Run(host + ":" + port); err != nil {
		panic(err)
	}
}

func PermissionCheckMiddleware(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestKey := c.GetHeader("API-KEY")
		if requestKey != apiKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "Invalid API key",
			})
			return
		}
		c.Next()
	}
}

func InnitRouter(apiKey string) *gin.Engine {
	router := gin.New()
	router.Use(ginzap.RecoveryWithZap(logger.ZapLogger, true))
	router.Use(ginzap.Ginzap(logger.ZapLogger, time.RFC3339Nano, true))
	router.Use(cors.Default())
	pprof.Register(router)

	apiGroup := router.Group("", PermissionCheckMiddleware(apiKey))
	apiGroup.POST("/image-task", handler.CreateGenerationTask)
	apiGroup.GET("/image", handler.GenerationImageFromGetRequest)

	apiGroup.POST("/upscale-task", handler.CreateUpscaleTask)
	apiGroup.GET("/upscale", handler.UpscaleImageFromGetRequest)

	apiGroup.POST("/describe-task", handler.CreateDescribeTask)
	return router
}
