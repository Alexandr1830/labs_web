package config

import "github.com/spf13/viper"

type MinioConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
	Bucket    string
	Region    string
}

func LoadMinioConfig() (*MinioConfig, error) {
	v := viper.New()
	v.SetConfigName("minio")
	v.SetConfigType("toml")
	v.AddConfigPath("../../config")
	v.AddConfigPath("config")
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg MinioConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
