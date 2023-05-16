package main

import (
	"flag"

	"github.com/haojie06/midjourney-http/internal/discordmd"
	"github.com/haojie06/midjourney-http/internal/logger"
	"github.com/haojie06/midjourney-http/internal/server"
	"github.com/spf13/viper"
)

func main() {
	configPath := flag.String("c", "", "config file path")
	flag.Parse()

	if *configPath != "" {
		viper.SetConfigFile(*configPath)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}
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
	apiKey := viper.GetString("server.apiKey")
	logger.Infof("service is starting, host: %s, port: %s", host, port)
	go discordmd.MidJourneyServiceApp.Start(midJourneyConfig)
	server.Start(host, port, apiKey)
}
