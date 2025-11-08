package sessionstore

import (
	"context"
	"database/sql"
)

type StoreInterface interface {
	AutoMigrate(ctx context.Context) error
	EnableDebug(debug bool)
	SessionExpiryGoroutine(ctx context.Context) error
	GetDB() *sql.DB

	// Old API
	Set(ctx context.Context, key string, value string, seconds int64, options SessionOptionsInterface) error
	Get(ctx context.Context, key string, defaultValue string, options SessionOptionsInterface) (string, error)
	GetMap(ctx context.Context, key string, defaultValue map[string]any, options SessionOptionsInterface) (map[string]any, error)
	GetAny(ctx context.Context, key string, defaultValue any, options SessionOptionsInterface) (any, error)
	Delete(ctx context.Context, key string, options SessionOptionsInterface) error
	Extend(ctx context.Context, key string, seconds int64, options SessionOptionsInterface) error
	Has(ctx context.Context, key string, options SessionOptionsInterface) (bool, error)
	MergeMap(ctx context.Context, key string, value map[string]any, seconds int64, options SessionOptionsInterface) error
	SetAny(ctx context.Context, key string, value any, seconds int64, options SessionOptionsInterface) error
	SetMap(ctx context.Context, key string, value map[string]any, seconds int64, options SessionOptionsInterface) error

	// New API
	SessionCount(ctx context.Context, query SessionQueryInterface) (int64, error)
	SessionCreate(ctx context.Context, session SessionInterface) error
	SessionDelete(ctx context.Context, session SessionInterface) error
	SessionDeleteByID(ctx context.Context, sessionID string) error
	SessionExtend(ctx context.Context, session SessionInterface, seconds int64) error
	SessionFindByID(ctx context.Context, sessionID string, options ...SessionOptionsInterface) (SessionInterface, error)
	SessionFindByKey(ctx context.Context, sessionKey string, options ...SessionOptionsInterface) (SessionInterface, error)
	SessionList(ctx context.Context, query SessionQueryInterface) ([]SessionInterface, error)
	SessionSoftDelete(ctx context.Context, session SessionInterface) error
	SessionSoftDeleteByID(ctx context.Context, sessionID string) error
	SessionUpdate(ctx context.Context, session SessionInterface) error
}
