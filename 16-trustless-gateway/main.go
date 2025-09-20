package main

import (
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	trustless "github.com/gosuda/boxo-starter-kit/16-trustless-gateway/pkg"
)

var (
	rootCmd = &cobra.Command{
		Use:   "indexer",
		Short: "trustless gateway server",
		Long:  "trustless gateway server",
		Run:   rootRun,
	}

	port     int
	upstream string
)

func init() {
	rootCmd.Flags().IntVarP(&port, "port", "p", 8080, "HTTP listen port")
	rootCmd.Flags().StringVarP(&upstream, "upstream", "u", "https://ipfs.io,https://dweb.link", "Comma-separated upstream trustless endpoints")
}

func rootRun(cmd *cobra.Command, args []string) {
	upstreams := splitAndTrim(upstream)
	if len(upstreams) == 0 {
		cmd.Help()
		log.Fatal().Msg("no upstreams specified")
	}

	gw, err := trustless.NewGatewayWrapper(port, upstreams)
	if err != nil {
		log.Fatal().Msgf("failed to create gateway: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info().Msgf("🚀 Trustless gateway listening on :%d", port)
		log.Info().Msgf("   Upstreams: %v", upstreams)
		errCh <- gw.Start()
	}()

	interrupt := handleKillSig(func() {
		if err := gw.Server.Shutdown(cmd.Context()); err != nil {
			log.Error().Msgf("failed to shutdown server: %v", err)
		}
	})
	<-interrupt.C
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Msgf("failed to execute command: %v", err)
	}
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

type interrupt struct {
	C chan struct{}
}

func handleKillSig(handler func()) interrupt {
	i := interrupt{
		C: make(chan struct{}),
	}

	sigChannel := make(chan os.Signal, 1)

	signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	go func() {
		for signal := range sigChannel {
			log.Info().Msgf("Receive signal %s, Shutting down...", signal)
			handler()
			close(i.C)
		}
	}()
	return i
}
