package bgpd

import (
	"context"
	"log/slog"

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
	<-ctx.Done()
	s.Stop()
	return nil
}
