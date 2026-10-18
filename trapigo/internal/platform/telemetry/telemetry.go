package telemetry

import (
	"context"
	"errors"
)

type Config struct {
	ServiceName   string
	Environment   string
	SamplingRatio float64
}

func Init(ctx context.Context, cfg Config) (func(ctx context.Context), error) {
	if cfg.ServiceName == "" {
		return nil, errors.New("telemetry service name must not be empty")
	}
	if cfg.Environment == "" {
		return nil, errors.New("telemetry environment must not be empty")
	}
	if cfg.SamplingRatio < 0 || cfg.SamplingRatio > 1 {
		return nil, errors.New("telemetry sampling ratio must be between 0 and 1")
	}

	return func(ctx context.Context) {
		// Cleanup logic here
	}, nil
}
