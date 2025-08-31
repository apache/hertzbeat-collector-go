package main

import (
	"flag"
	"os"

	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector"
)

var (
	conf string
)

func init() {
	flag.StringVar(&conf, "conf", "hertzbeat-collector.yaml", "path to config file")
}

func main() {

	if err := collector.Bootstrap(conf); err != nil {

		os.Exit(1)
	}
}
