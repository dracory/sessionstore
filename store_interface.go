package sessionstore

import "context"

type StoreInterface interface {
	AutoMigrate(ctx context.Context) error
	EnableDebug(debug bool)
	SessionExpiryGoroutine(ctx context.Context) error

	// New API
	SessionCount(ctx context.Context, query SessionQueryInterface) (int64, error)
	SessionCreate(ctx context.Context, session SessionInterface) error
	SessionDelete(ctx context.Context, session SessionInterface) error
	SessionDeleteByID(ctx context.Context, sessionID string) error
	SessionExtend(ctx context.Context, session SessionInterface, seconds int64) error
	SessionFindByID(ctx context.Context, sessionID string) (SessionInterface, error)
	SessionFindByKey(ctx context.Context, sessionKey string) (SessionInterface, error)
	SessionList(ctx context.Context, query SessionQueryInterface) ([]SessionInterface, error)
	SessionSoftDelete(ctx context.Context, session SessionInterface) error
	SessionSoftDeleteByID(ctx context.Context, sessionID string) error
	SessionUpdate(ctx context.Context, session SessionInterface) error
}
