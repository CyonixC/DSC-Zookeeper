package configReader

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"sync"
)

type Config struct {
	Servers []string `json:"servers"`
	Clients []string `json:"clients"`
}

var config_instance Config
var once sync.Once

func GetConfig() *Config {
	once.Do(func() {
		config_instance = loadConfig("config.json")
	})
	return &config_instance
}

func loadConfig(filename string) Config {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Failed to open config file: %v", err)
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	var config Config
	if err := json.Unmarshal(bytes, &config); err != nil {
		log.Fatalf("Failed to parse config file: %v", err)
	}

	return config
}
