package main

import (
	"encoding/json"
	"log"
	"os"
)

type Configuration struct {
	Database struct {
		DbName     string
		DbUser     string
		DbPassword string
		DbSock     string
	}
	Polling struct {
		IntervalMillis int
	}
	Notifications struct {
		TimeoutMillis int
		Emails        []string
		From          string
		Password      string
		Server        string
		Port          int
	}
	Users map[string]string
}

const (
	configLocation = "/etc/garagebotrc.json"
)

func readConfiguration() *Configuration {
	file, err := os.Open(configLocation)
	if err != nil {
		log.Panic("Could not open config file:", err)
	}
	decoder := json.NewDecoder(file)
	configuration := &Configuration{}
	err = decoder.Decode(configuration)
	if err != nil {
		log.Panic("Could not parse config file:", err)
	}
	return configuration
}
