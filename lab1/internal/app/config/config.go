package config

import (
	"os"
	
   "github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Конфиг базы данных
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

// Общий конфиг приложения
type Config struct {
	ServiceHost string
	ServicePort int
	Database    DatabaseConfig
}

func NewConfig() (*Config, error) {
	var err error

   configName := "config"
   _ = godotenv.Load()
   if os.Getenv("CONFIG_NAME") != "" {
      configName = os.Getenv("CONFIG_NAME")
   }

   viper.SetConfigName(configName)
   viper.SetConfigType("toml")
   viper.AddConfigPath("../../config")
   viper.AddConfigPath("config")
   viper.AddConfigPath(".")
   viper.WatchConfig()

	err = viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	cfg := &Config{} // создаем объект конфига
	err = viper.Unmarshal(cfg) // читаем информацию из файла, 
	// конвертируем и затем кладем в нашу переменную cfg
	if err != nil {
		return nil, err
	}

	log.Info("config parsed")

	return cfg, nil
}