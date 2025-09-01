package main

import (
	"flag"
	"os"

	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	ConfPath string
	Version  string
)

func init() {
	flag.StringVar(&ConfPath, "conf", "hertzbeat-collector.yaml", "path to config file, eg: -conf ./hertzbeat-collector.yaml")
}

func main() {

	if err := collector.Bootstrap(ConfPath, Version); err != nil {

		os.Exit(1)
	}
}
