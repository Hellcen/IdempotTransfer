package config

import (
	"fmt"
	"log"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig `yaml:"Server"`
	DB     DBConfig     `yaml:"DB"`
	Token  TokenConfig  `yaml:"Token"`
	Logger LoggerConfig `yaml:"Logger"`
}

type ServerConfig struct {
	Port string `yaml:"port" default:"8080"`
}

type DBConfig struct {
	DatabaseURL        string        `yaml:"databaseURL"`
	Port               int           `yaml:"port" default:"5432"`
	MaxOpenConnection  int           `yaml:"maxOpenConnection" default:"15"`
	MaxIdleConnection  int           `yaml:"maxIdleConnection" default:"10"`
	ConnectionLifetime time.Duration `yaml:"connectionLifetime" default:"3600"`
}

type TokenConfig struct {
	AuthToken string `yaml:"authToken" default:"test-token"`
}

type LoggerConfig struct {
	LoggerLevel string `yaml:"loggerLevel" default:"info"`
}

func Load() (*Config, error) {
	viper.AutomaticEnv()

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./internal/config")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("file not found")
		} else {
			log.Println("error reading config file")
		}
	} else {
		log.Printf("using config file: %s", viper.ConfigFileUsed())
	}

	log.Printf("all settings: %v", viper.AllSettings())

	var config Config

	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode into struct: %w", err)
	}

	return &config, nil
}
