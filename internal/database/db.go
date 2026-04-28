package database

import (
	"context"
	"errors"
	"time"

	"github.com/jcroyoaun/totalcompmx/assets"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jmoiron/sqlx"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/lib/pq"
)

const defaultTimeout = 3 * time.Second

type migrationRunner interface {
	Up() error
}

var connectDB = sqlx.ConnectContext
var newMigrationSource = iofs.New
var newMigrationRunner = func(sourceName string, sourceDriver source.Driver, databaseURL string) (migrationRunner, error) {
	return migrate.NewWithSourceInstance(sourceName, sourceDriver, databaseURL)
}

type DB struct {
	dsn string
	*sqlx.DB
}

func New(dsn string) (*DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	db, err := connectDB(ctx, "postgres", "postgres://"+dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(2 * time.Hour)

	return &DB{dsn: dsn, DB: db}, nil
}

func (db *DB) MigrateUp() error {
	iofsDriver, err := newMigrationSource(assets.EmbeddedFiles, "migrations")
	if err != nil {
		return err
	}

	migrator, err := newMigrationRunner("iofs", iofsDriver, "postgres://"+db.dsn)
	if err != nil {
		return err
	}

	return ignoreNoMigrationChange(migrator.Up())
}

func ignoreNoMigrationChange(err error) error {
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}

	return err
}
