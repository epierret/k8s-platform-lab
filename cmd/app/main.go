package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/epierret/k8s-platform-lab/internal/app"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	info := app.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}

	srv := app.NewServer(info, logger)
	httpSrv := &http.Server{
		Addr:              ":8080",
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

	go func() {
		logger.Info("server_starting", "addr", httpSrv.Addr, "version", version)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server_error", "err", err)
			os.Exit(1)
		}
	}()

	srv.SetReady(true)

	<-ctx.Done()
	logger.Info("shutdown_signal_received")
	srv.SetReady(false)
	time.Sleep(5 * time.Second)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	logger.Info("server_stopped")
}
