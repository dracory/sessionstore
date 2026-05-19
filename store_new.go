package sessionstore

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/dracory/sb"
)

// NewStoreOptions define the options for creating a new session store
type NewStoreOptions struct {
	SessionTableName   string
	DB                 *sql.DB
	DbDriverName       string
	TimeoutSeconds     int64
	AutomigrateEnabled bool
	DebugEnabled       bool
	SqlLogger          *slog.Logger
	EncryptionEnabled  bool
	EncryptionKey      []byte
}

// NewStore creates a new session store
func NewStore(opts NewStoreOptions) (StoreInterface, error) {
	store := &storeImplementation{
		sessionTableName:   opts.SessionTableName,
		automigrateEnabled: opts.AutomigrateEnabled,
		db:                 opts.DB,
		dbDriverName:       opts.DbDriverName,
		debugEnabled:       opts.DebugEnabled,
		timeoutSeconds:     opts.TimeoutSeconds,
		sqlLogger:          opts.SqlLogger,
	}

	if opts.EncryptionEnabled && len(opts.EncryptionKey) == 0 {
		return nil, errors.New("session store: encryption key is required when encryption is enabled")
	}

	if opts.EncryptionEnabled {
		encryptor, err := newSessionEncryptor(opts.EncryptionKey)

		if err != nil {
			return nil, err
		}

		store.encryptor = encryptor
	}

	if store.sessionTableName == "" {
		return nil, errors.New("session store: sessionTableName is required")
	}

	if store.db == nil {
		return nil, errors.New("session store: DB is required")
	}

	if store.dbDriverName == "" {
		store.dbDriverName = sb.DatabaseDriverName(store.db)
	}

	if store.sqlLogger == nil {
		store.sqlLogger = slog.Default()
	}

	if store.timeoutSeconds <= 0 {
		store.timeoutSeconds = 2 * 60 * 60 // 2 hours
	}

	if store.automigrateEnabled {
		store.MigrateUp(context.Background())
	}

	return store, nil
}
