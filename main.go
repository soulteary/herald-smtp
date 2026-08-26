package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
	"github.com/soulteary/herald-smtp/internal/config"
	"github.com/soulteary/herald-smtp/internal/handler"
	"github.com/soulteary/herald-smtp/internal/router"
	"github.com/soulteary/logger-kit/v2"
	version "github.com/soulteary/version-kit/v2"
)

// showBanner displays the startup banner with version
func showBanner() {
	pterm.DefaultBox.Println(
		putils.CenterText(
			"Herald SMTP\n" +
				"SMTP Email Provider for Herald\n" +
				"Version: " + version.Version,
		),
	)
	time.Sleep(time.Millisecond)
}

func main() {
	showBanner()

	level := logger.ParseLevelFromEnv("LOG_LEVEL", logger.InfoLevel)
	log := logger.New(logger.Config{
		Level:          level,
		ServiceName:    "herald-smtp",
		ServiceVersion: version.Version,
	})

	port := config.Port
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}
	if !config.Valid() {
		log.Warn().Msg("SMTP not configured; /v1/send will return 503")
	}
	app := newApp()
	router.Setup(app, log)

	go func() {
		if err := app.Listen(port); err != nil {
			log.Fatal().Err(err).Msg("listen failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Info().Msg("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Warn().Err(err).Msg("shutdown error")
	}
}

func newApp() *fiber.App {
	return fiber.New(fiber.Config{
		BodyLimit:    config.HTTPBodyLimit(),
		ReadTimeout:  config.HTTPReadTimeout(),
		WriteTimeout: config.HTTPWriteTimeout(),
		IdleTimeout:  config.HTTPIdleTimeout(),
		ErrorHandler: handler.FrameworkErrorHandler,
	})
}
