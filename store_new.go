package sessionstore

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"

	"github.com/dracory/neat"
)

// NewStoreOptions defines the options for creating a new session store.
type NewStoreOptions struct {
	SessionTableName   string
	DB                 *sql.DB
	TimeoutSeconds     int64
	AutomigrateEnabled bool
	DebugEnabled       bool
	EncryptionEnabled  bool
	EncryptionKey      []byte
}

// NewStore creates a new session store.
func NewStore(opts NewStoreOptions) (StoreInterface, error) {
	if opts.DB == nil {
		return nil, errors.New("session store: DB is required")
	}

	if opts.SessionTableName == "" {
		return nil, errors.New("session store: sessionTableName is required")
	}

	if opts.EncryptionEnabled && len(opts.EncryptionKey) == 0 {
		return nil, errors.New("session store: encryption key is required when encryption is enabled")
	}

	neatDB, err := neat.NewFromSQLDB(opts.DB)
	if err != nil {
		return nil, err
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	store := &storeImplementation{
		sessionTableName:   opts.SessionTableName,
		db:                 neatDB,
		automigrateEnabled: opts.AutomigrateEnabled,
		debugEnabled:       opts.DebugEnabled,
		timeoutSeconds:     opts.TimeoutSeconds,
		logger:             logger,
	}

	if opts.EncryptionEnabled {
		encryptor, err := newSessionEncryptor(opts.EncryptionKey)
		if err != nil {
			return nil, err
		}
		store.encryptor = encryptor
	}

	if store.timeoutSeconds <= 0 {
		store.timeoutSeconds = 2 * 60 * 60 // 2 hours
	}

	if store.automigrateEnabled {
		if err := store.MigrateUp(context.Background()); err != nil {
			return nil, err
		}
	}

	return store, nil
}
