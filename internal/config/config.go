package config

import (
	"flag"
	"log"

	"github.com/caarlos0/env"
)

var (
	runAddress           string
	databaseURI          string
	accrualSystemAddress string
)

type ServerConfig struct {
	RunAddress           string `env:"RUN_ADDRESS" envDefault:"localhost:8888"`
	DatabaseURI          string `env:"DATABASE_URI" envDefault:"postgres://gophermart:gophermart@127.0.0.1:5432/gophermart"`
	AccrualSystemAddress string `env:"ACCRUAL_SYSTEM_ADDRESS" envDefault:"http://localhost:8080"`
}

func DefaultServerConfig() *ServerConfig {
	cfg := &ServerConfig{
		RunAddress:           runAddress,
		DatabaseURI:          databaseURI,
		AccrualSystemAddress: accrualSystemAddress,
	}

	if err := env.Parse(cfg); err != nil {
		log.Fatal(err)
	}
	return cfg
}

func init() {
	flag.StringVar(&runAddress, "a", "", "server address")
	flag.StringVar(&databaseURI, "d", "", "database data source name")
	flag.StringVar(&accrualSystemAddress, "r", "", "accrual system address")
}
