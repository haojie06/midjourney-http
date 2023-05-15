package main

import (
	"github.com/haojie06/midjourney-http/internal/discordmd"
	"github.com/haojie06/midjourney-http/internal/logger"
	"github.com/haojie06/midjourney-http/internal/server"
	"github.com/spf13/viper"
)

func main() {
	viper.AddConfigPath(".")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	}
	var midJourneyConfig discordmd.MidJourneyServiceConfig
	if err := viper.UnmarshalKey("midJourney", &midJourneyConfig); err != nil {
		panic(err)
	}
	viper.SetDefault("server.host", "127.0.0.1")
	viper.SetDefault("server.port", "9000")
	host := viper.GetString("server.host")
	port := viper.GetString("server.port")
	logger.Infof("service is starting, host: %s, port: %s", host, port)
	go discordmd.MidJourneyServiceApp.Start(midJourneyConfig)
	server.Start(host, port)
}
