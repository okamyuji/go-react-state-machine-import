// Command serverはImportJob HTTP APIを起動するエントリポイントです。
// ここでRepository / Clock / IDSourceの具象実装をServiceにワイヤリングします。
// それ以外の層はすべてinterfaceにのみ依存しているので、実装を差し替えても
// このファイルにしか変更が入りません。
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/okamyuji/go-react-state-machine-import/backend/internal/httpapi"
	"github.com/okamyuji/go-react-state-machine-import/backend/internal/importjob"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	addr := flag.String("addr", ":8080", "HTTP リッスンアドレス")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	repo := importjob.NewInMemoryRepository()
	svc := importjob.NewService(repo, importjob.SystemClock{}, importjob.NewSequentialID("job"))
	handler := httpapi.NewHandler(svc, logger)

	server := &http.Server{
		Addr:              *addr,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("サーバ起動", slog.String("addr", *addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case <-ctx.Done():
		logger.Info("サーバを停止します")
	case err := <-serverErr:
		if err != nil {
			logger.Error("サーバエラー", slog.String("err", err.Error()))
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("シャットダウンエラー", slog.String("err", err.Error()))
		return err
	}
	return nil
}
