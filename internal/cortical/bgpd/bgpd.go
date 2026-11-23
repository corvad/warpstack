package bgpd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	api "github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/pkg/server"
)

const DefaultAPIAddress = "127.0.0.1:50051"

func Run(ctx context.Context, logger *slog.Logger, apiAddress string) error {
	level := new(slog.LevelVar)
	s := server.NewBgpServer(
		server.GrpcListenAddress(apiAddress),
		server.LoggerOption(logger, level),
	)
	go s.Serve()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.Stop()
			return nil
		case <-ticker.C:
			if _, err := s.GetBgp(ctx, &api.GetBgpRequest{}); err != nil && ctx.Err() == nil {
				return fmt.Errorf("speaker is no longer serving: %w", err)
			}
		}
	}
}
