package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/corvad/warpstack/internal/cortical/supervisor"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	s, err := supervisor.New([]supervisor.Spec{
		{
			Subsystem: &test{name: "test1"},
			Options: supervisor.Options{
				MinBackoff:        1 * time.Second,
				MaxBackoff:        10 * time.Second,
				StableBackoff:     1 * time.Minute,
				BackoffMultiplier: 2.0,
			},
		},
		{
			Subsystem: &test{name: "test2"},
			Options: supervisor.Options{
				MinBackoff:        1 * time.Second,
				MaxBackoff:        10 * time.Second,
				StableBackoff:     1 * time.Minute,
				BackoffMultiplier: 4.0,
			},
		},
		{
			Subsystem: &test{name: "test3"},
			Options: supervisor.Options{
				MinBackoff:        1 * time.Second,
				MaxBackoff:        10 * time.Second,
				StableBackoff:     1 * time.Minute,
				BackoffMultiplier: 2.0,
			},
		},
	})
	if err != nil {
		log.Panicf("[main] failed to create supervisor: %v", err)
	}
	s.Init(ctx)
	s.Run(ctx)
}
