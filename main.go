package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/hbomb79/Thea/internal"
	"github.com/hbomb79/Thea/pkg/logger"
)

var log = logger.Get("Bootstrap")

const VERSION = 1.0

func main() {
	var (
		conf              *internal.TheaConfig = &internal.TheaConfig{}
		defaultConfigPath                      = filepath.Join(conf.GetConfigDir(), "/config.toml")
		helpFlag                               = flag.Bool("help", false, "Whether to display help information")
		configFlag                             = flag.String("config", "", fmt.Sprintf("The path to the config file that Thea can load, defaults to '%s'", defaultConfigPath))
	)

	flag.Parse()

	if *helpFlag {
		flag.Usage()
	} else {
		var path string
		if *configFlag != "" {
			path = *configFlag
		} else {
			path = defaultConfigPath
		}

		if err := conf.LoadFromFile(path); err != nil {
			panic(err)
		}

		startThea(conf)
	}
}

func startThea(config *internal.TheaConfig) {
	log.Emit(logger.INFO, " --- Starting Thea (version %v) ---\n", VERSION)

	ctx, ctxCancel := context.WithCancel(context.Background())
	go listenForInterrupt(ctxCancel)

	if err := internal.New(*config).Run(ctx); err != nil {
		log.Emit(logger.FATAL, "Failed to start Thea: %v", err.Error())

		return
	}

	log.Emit(logger.STOP, "Thea shutdown complete\n")
}

func listenForInterrupt(ctxCancel context.CancelFunc) {
	exitChannel := make(chan os.Signal, 1)
	signal.Notify(exitChannel, os.Interrupt, syscall.SIGTERM)

	<-exitChannel
	ctxCancel()
}
