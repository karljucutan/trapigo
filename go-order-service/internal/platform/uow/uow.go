package uow

import (
	"context"
	"database/sql"
)

type UnitOfWork[T any] interface {
	RunInTx(ctx context.Context, fn func(T) error) error
}

type SQLUnitOfWork[T any] struct {
	db      *sql.DB
	factory func(*sql.Tx) T
}

func NewSQLUnitOfWork[T any](db *sql.DB, factory func(*sql.Tx) T) *SQLUnitOfWork[T] {
	return &SQLUnitOfWork[T]{
		db:      db,
		factory: factory,
	}
}

func (u *SQLUnitOfWork[T]) RunInTx(ctx context.Context, fn func(T) error) error {
	tx, err := u.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			panic(r)
		}
	}()

	txrepositories := u.factory(tx)
	if err := fn(txrepositories); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}
