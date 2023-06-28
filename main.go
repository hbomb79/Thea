package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/hbomb79/Thea/internal"
	"github.com/hbomb79/Thea/pkg/logger"
)

var log = logger.Get("Bootstrap")

const VERSION = 1.0

type Thea interface {
	Start(ctx context.Context) error
}

func listenForInterrupt(ctxCancel context.CancelFunc) {
	exitChannel := make(chan os.Signal, 1)
	signal.Notify(exitChannel, os.Interrupt, syscall.SIGTERM)

	<-exitChannel
	ctxCancel()
}

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	conf := internal.TheaConfig{}
	if err := conf.LoadFromFile(filepath.Join(homeDir, ".config/thea/config.yaml")); err != nil {
		panic(err)
	}

	log.Emit(logger.INFO, " --- Starting Thea (version %v) ---\n", VERSION)

	ctx, ctxCancel := context.WithCancel(context.Background())
	go listenForInterrupt(ctxCancel)

	if err := internal.New(conf).Run(ctx); err != nil {
		log.Emit(logger.FATAL, "Failed to start Thea: %v", err.Error())

		return
	}

	log.Emit(logger.STOP, "Thea shutdown complete\n")
}
