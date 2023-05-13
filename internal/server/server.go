package server

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/haojie06/midjourney-http/internal/server/handler"
)

func Start(host, port string) {
	router := InnitRouter()

	if err := router.Run(host + ":" + port); err != nil {
		panic(err)
	}
}

func InnitRouter() *gin.Engine {
	router := gin.Default()
	router.Use(cors.Default())
	pprof.Register(router)
	router.POST("/generation-task", handler.CreateGenerationTask)
	router.GET("/image", handler.GenerationImageFromGetRequest)
	return router
}
