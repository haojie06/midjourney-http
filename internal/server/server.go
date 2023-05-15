package server

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/pprof"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/haojie06/midjourney-http/internal/logger"
	"github.com/haojie06/midjourney-http/internal/server/handler"
)

func Start(host, port string) {
	router := InnitRouter()

	if err := router.Run(host + ":" + port); err != nil {
		panic(err)
	}
}

func InnitRouter() *gin.Engine {
	router := gin.New()
	router.Use(ginzap.RecoveryWithZap(logger.ZapLogger, true))
	router.Use(ginzap.Ginzap(logger.ZapLogger, time.RFC3339Nano, true))
	router.Use(cors.Default())
	pprof.Register(router)
	router.POST("/generation-task", handler.CreateGenerationTask)
	router.GET("/image", handler.GenerationImageFromGetRequest)
	return router
}
