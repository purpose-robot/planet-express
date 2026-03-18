package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

func (app *application) listenAndServe(ctx context.Context) error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.HTTP.Port),
		Handler:      app.routes(),
		ErrorLog:     slog.NewLogLogger(app.logger.Handler(), slog.LevelDebug),
		IdleTimeout:  app.config.HTTP.IdleTimeout,
		ReadTimeout:  app.config.HTTP.ReadTimeout,
		WriteTimeout: app.config.HTTP.WriteTimeout,
	}

	shutdownErrorChan := make(chan error)

	go func() {
		<-ctx.Done()

		app.logger.Info("stopping server", slog.Group("server", slog.String("address", srv.Addr)))

		shutdownCtx, cancel := context.WithTimeout(context.Background(), app.config.HTTP.ShutdownTimeout)
		defer cancel()

		shutdownErrorChan <- srv.Shutdown(shutdownCtx)
	}()

	app.logger.InfoContext(ctx, "starting API server", slog.Group("server", slog.String("address", srv.Addr)))

	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	err = <-shutdownErrorChan
	if err != nil {
		return err
	}

	app.logger.Info("API server successfully stopped", slog.Group("server", slog.String("address", srv.Addr)))

	return nil
}
