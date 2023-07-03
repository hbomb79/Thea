package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/hbomb79/Thea/internal"
	"github.com/hbomb79/Thea/pkg/logger"
)

const VERSION = 1.0

var (
	log = logger.Get("Bootstrap")

	conf         *internal.TheaConfig = &internal.TheaConfig{}
	logLevelFlag                      = flag.String("log-level", "info", "Define logging level from one of [verbose, debug, info, important, warning, error]")
	helpFlag                          = flag.Bool("help", false, "Whether to display help information")
	configFlag                        = flag.String("config", filepath.Join(conf.GetConfigDir(), "/config.toml"), "The path to the config file that Thea will load")
)

func main() {
	flag.Parse()

	level, err := parseLogLevelFromString(*logLevelFlag)
	if err != nil {
		fmt.Println(err.Error())
		flag.Usage()

		return
	}
	logger.SetMinLoggingLevel(level)

	if *helpFlag {
		flag.Usage()
	} else {
		log.Emit(logger.DEBUG, "Loading configuration from '%s'\n", *configFlag)
		if err := conf.LoadFromFile(*configFlag); err != nil {
			panic(err)
		}

		startThea(conf)
	}
}

func startThea(config *internal.TheaConfig) {
	log.Emit(logger.INFO, " --- Starting Thea (version %.1f) ---\n", VERSION)

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

func parseLogLevelFromString(l string) (logger.LogLevel, error) {
	switch strings.ToLower(l) {
	case "verbose":
		return logger.VERBOSE.Level(), nil
	case "debug":
		return logger.DEBUG.Level(), nil
	case "info":
		return logger.INFO.Level(), nil
	case "important":
		return logger.SUCCESS.Level(), nil
	case "warning":
		return logger.WARNING.Level(), nil
	case "error":
		return logger.ERROR.Level(), nil
	default:
		return logger.INFO.Level(), fmt.Errorf("logging level %s is not recognized", l)
	}
}
