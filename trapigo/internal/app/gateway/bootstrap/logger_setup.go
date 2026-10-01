package bootstrap

import (
	"log/slog"
	"os"
	"strings"

	configuration "github.com/karljucutan/trapigo/trapigo/pkg"
)

func setDefaultLogger() {
	level := slog.LevelInfo

	if configuredLevel := strings.TrimSpace(strings.ToUpper(configuration.GetEnv("LOG_LEVEL", ""))); configuredLevel != "" {
		switch configuredLevel {
		case "DEBUG":
			level = slog.LevelDebug
		case "INFO":
			level = slog.LevelInfo
		case "WARN":
			level = slog.LevelWarn
		case "ERROR":
			level = slog.LevelError
		}
	}

	slog.SetDefault(slog.New(
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}),
	).With("app", "trapigo"))
}
