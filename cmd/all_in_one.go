package cmd

import (
	"context"

	"github.com/cryptolink/cryptolink/internal/app"
	"github.com/cryptolink/cryptolink/pkg/graceful"
	"github.com/spf13/cobra"
)

var allInOneCommand = &cobra.Command{
	Use:   "all-in-one",
	Short: "Runs server and scheduler in a single instance",
	Run:   allInOne,
}

func allInOne(_ *cobra.Command, _ []string) {
	ctx := context.Background()

	cfg := resolveConfig()

	service := app.New(ctx, cfg)

	setupOnBeforeRun(service, cfg)

	service.RunServer()
	service.RunScheduler()

	if err := graceful.WaitShutdown(); err != nil {
		service.Logger().Error().Err(err).Msg("unable to shutdown service gracefully")
		return
	}

	service.Logger().Info().Msg("shutdown complete")
}
