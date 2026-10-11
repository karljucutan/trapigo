package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	openTimeout   = 30 * time.Second
	retryInterval = 2 * time.Second
)

func Open(dataSourceName string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("open pgx: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), openTimeout)
	defer cancel()

	db.Ping()
	if err := pingWithRetry(ctx, db.PingContext, retryInterval); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database after retry timeout (%s): %w", openTimeout, err)
	}

	return db, nil
}

func pingWithRetry(ctx context.Context, ping func(context.Context) error, interval time.Duration) error {
	var lastErr error
	for {
		if err := ping(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return lastErr
		case <-timer.C:
		}
	}
}
