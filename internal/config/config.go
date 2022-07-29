package config

import (
	"fmt"

	"github.com/caarlos0/env/v6"
)

type Config struct {
	PostgresPassword       string `env:"POSTGRES_PASSWORD"`
	PostgresUser           string `env:"POSTGRES_USER"`
	PostgresDB             string `env:"POSTGRES_DB"`
	PostgresHost           string `env:"POSTGRES_HOST"`
	PostgresPort           string `env:"POSTGRES_PORT"`
	PositionServicePortRPC string `env:"POSITION_SERVICE_PORT_RPC"`
	PositionServiceHostRPC string `env:"POSITION_SERVICE_HOST_RPC"`
	PriceServicePortRPC    string `env:"PRICE_SERVICE_PORT_RPC"`
	PriceServiceHostRPC    string `env:"PRICE_SERVICE_HOST_RPC"`
}

func (c *Config) GetConnStringPostgres() string {
	return fmt.Sprintf("postgres://%v:%v@%v:%v/%v", c.PostgresUser, c.PostgresPassword, c.PostgresHost, c.PostgresPort, c.PostgresDB)
}

func (c *Config) GetConnStringToPriceService() string {
	return fmt.Sprintf("%s:%s", c.PriceServiceHostRPC, c.PriceServicePortRPC)
}
func (c *Config) GetConnStringToPositionService() string {
	return fmt.Sprintf("%s:%s", c.PositionServiceHostRPC, c.PositionServicePortRPC)
}

func GetConfig() (*Config, error) {
	conf := Config{}
	err := env.Parse(&conf)
	if err != nil {
		return nil, err
	}
	return &conf, nil
}
