package sessionstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/dracory/neat"
	contractsorm "github.com/dracory/neat/contracts/database/orm"
	contractsschema "github.com/dracory/neat/contracts/database/schema"
	"github.com/dromara/carbon/v2"
	"github.com/samber/lo"
	"github.com/spf13/cast"
)

// == INTERFACE ===============================================================

type StoreInterface interface {
	// GetSessionTableName returns the session table name
	GetSessionTableName() string
	// SetSessionTableName sets the session table name
	SetSessionTableName(sessionTableName string)
	// GetTimeoutSeconds returns the session timeout in seconds
	GetTimeoutSeconds() int64
	// SetTimeoutSeconds sets the session timeout in seconds
	SetTimeoutSeconds(timeoutSeconds int64)

	// MigrateDown drops the session table
	MigrateDown(ctx context.Context, tx ...*sql.Tx) error
	// MigrateUp creates the session table
	MigrateUp(ctx context.Context, tx ...*sql.Tx) error

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

// == TYPE ====================================================================

var _ StoreInterface = (*storeImplementation)(nil)

// storeImplementation implements StoreInterface for session operations.
type storeImplementation struct {
	sessionTableName   string
	db                 *neat.Database
	timeoutSeconds     int64
	automigrateEnabled bool
	debugEnabled       bool
	logger             *slog.Logger
	encryptor          *sessionEncryptor
}

// == MIGRATE =================================================================

// MigrateUp creates the session table if it does not already exist.
func (st *storeImplementation) MigrateUp(ctx context.Context, tx ...*sql.Tx) error {
	if st.db.Schema().HasTable(st.sessionTableName) {
		if st.debugEnabled {
			st.logger.Info("MigrateUp: table already exists", "table", st.sessionTableName)
		}
		return nil
	}

	err := st.db.Schema().Create(st.sessionTableName, func(table contractsschema.Blueprint) {
		table.String(COLUMN_ID, 21)
		table.Primary(COLUMN_ID)
		table.String(COLUMN_SESSION_KEY, 255)
		table.String(COLUMN_USER_ID, 40)
		table.String(COLUMN_IP_ADDRESS, 50)
		table.String(COLUMN_USER_AGENT, 1024)
		table.Text(COLUMN_SESSION_VALUE)
		table.DateTime(COLUMN_EXPIRES_AT)
		table.DateTime(COLUMN_CREATED_AT)
		table.DateTime(COLUMN_UPDATED_AT)
		table.DateTime(COLUMN_SOFT_DELETED_AT)
	})

	if err != nil {
		if st.debugEnabled {
			st.logger.Error("MigrateUp failed", "error", err)
		}
		return err
	}

	return nil
}

// MigrateDown drops the session table.
func (st *storeImplementation) MigrateDown(ctx context.Context, tx ...*sql.Tx) error {
	if !st.db.Schema().HasTable(st.sessionTableName) {
		if st.debugEnabled {
			st.logger.Info("MigrateDown: table does not exist", "table", st.sessionTableName)
		}
		return nil
	}

	err := st.db.Schema().Drop(st.sessionTableName)
	if err != nil {
		if st.debugEnabled {
			st.logger.Error("MigrateDown failed", "error", err)
		}
		return err
	}
	return nil
}

// == DEBUG ===================================================================

// EnableDebug enables or disables debug mode.
func (st *storeImplementation) EnableDebug(debug bool) {
	st.debugEnabled = debug
	if debug {
		st.db.EnableDebug()
		st.logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	} else {
		st.db.DisableDebug()
		st.logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}
}

// == TABLE NAME ==============================================================

// GetSessionTableName returns the session table name.
func (st *storeImplementation) GetSessionTableName() string {
	return st.sessionTableName
}

// SetSessionTableName sets the session table name.
func (st *storeImplementation) SetSessionTableName(sessionTableName string) {
	st.sessionTableName = sessionTableName
}

// == TIMEOUT =================================================================

// GetTimeoutSeconds returns the session timeout in seconds.
func (st *storeImplementation) GetTimeoutSeconds() int64 {
	return st.timeoutSeconds
}

// SetTimeoutSeconds sets the session timeout in seconds.
func (st *storeImplementation) SetTimeoutSeconds(timeoutSeconds int64) {
	st.timeoutSeconds = timeoutSeconds
}

// == DB ======================================================================

// GetDB returns the underlying *sql.DB.
func (st *storeImplementation) GetDB() *sql.DB {
	db, _ := st.db.DB()
	return db
}

// == SESSION EXPIRY GOROUTINE ================================================

// SessionExpiryGoroutine deletes expired sessions periodically (every minute).
// It honors the provided context for cancellation.
func (st *storeImplementation) SessionExpiryGoroutine(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := st.expireSessionsOnce(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := st.expireSessionsOnce(ctx); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				return err
			}
		}
	}
}

func (st *storeImplementation) expireSessionsOnce(ctx context.Context) error {
	if st.debugEnabled {
		st.logger.Debug("Cleaning expired sessions...")
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	now := carbon.Now(carbon.UTC).ToDateTimeString(carbon.UTC)
	_, err := st.db.Query().
		Model(&sessionImplementation{}).
		Table(st.sessionTableName).
		Where(COLUMN_EXPIRES_AT+" < ?", now).
		Delete()

	if err != nil {
		if errors.Is(err, context.Canceled) {
			return context.Canceled
		}
		st.logger.Error("SessionExpiryGoroutine error", "error", err)
	}
	return nil
}

// == OLD API =================================================================

// Extend extends the session expiry time by the given seconds.
func (store *storeImplementation) Extend(ctx context.Context, sessionKey string, seconds int64, options SessionOptionsInterface) error {
	session, err := store.SessionFindByKey(ctx, sessionKey, options)
	if err != nil {
		return err
	}
	if session == nil {
		return errors.New("session not found")
	}

	expiresAt := carbon.Now(carbon.UTC).AddSeconds(cast.ToInt(seconds)).ToDateTimeString(carbon.UTC)
	session.SetExpiresAt(expiresAt)
	return store.SessionUpdate(ctx, session)
}

// Delete deletes a session by key with optional filters from options.
func (st *storeImplementation) Delete(ctx context.Context, sessionKey string, options SessionOptionsInterface) error {
	q := st.db.Query().Table(st.sessionTableName).Where(COLUMN_SESSION_KEY+" = ?", sessionKey)

	if options.HasUserAgent() {
		q = q.Where(COLUMN_USER_AGENT+" = ?", options.GetUserAgent())
	}
	if options.HasUserID() {
		q = q.Where(COLUMN_USER_ID+" = ?", options.GetUserID())
	}
	if options.HasIPAddress() {
		q = q.Where(COLUMN_IP_ADDRESS+" = ?", options.GetIPAddress())
	}

	_, err := q.Delete()
	return err
}

// Get returns the value for the session with the given key, or defaultValue.
func (st *storeImplementation) Get(ctx context.Context, sessionKey string, defaultValue string, options SessionOptionsInterface) (string, error) {
	session, err := st.SessionFindByKey(ctx, sessionKey, options)
	if err != nil {
		return "", err
	}
	if session != nil {
		decrypted, err := st.decryptValue(session.GetValue())
		if err != nil {
			return "", err
		}
		return decrypted, nil
	}
	return defaultValue, nil
}

// GetAny attempts to parse the value as interface{}, use with SetAny.
func (st *storeImplementation) GetAny(ctx context.Context, key string, defaultValue any, options SessionOptionsInterface) (any, error) {
	session, err := st.SessionFindByKey(ctx, key, options)
	if err != nil {
		return defaultValue, err
	}
	if session != nil {
		jsonValue, err := st.decryptValue(session.GetValue())
		if err != nil {
			return defaultValue, err
		}
		var val any
		if err := json.Unmarshal([]byte(jsonValue), &val); err != nil {
			return defaultValue, err
		}
		return val, nil
	}
	return defaultValue, nil
}

// GetMap attempts to parse the value as map[string]any, use with SetMap.
func (st *storeImplementation) GetMap(ctx context.Context, key string, defaultValue map[string]any, options SessionOptionsInterface) (map[string]any, error) {
	session, err := st.SessionFindByKey(ctx, key, options)
	if err != nil {
		return defaultValue, err
	}
	if session != nil {
		jsonValue, err := st.decryptValue(session.GetValue())
		if err != nil {
			return defaultValue, err
		}
		var val map[string]any
		if err := json.Unmarshal([]byte(jsonValue), &val); err != nil {
			return defaultValue, err
		}
		return val, nil
	}
	return defaultValue, nil
}

// Has checks if a session exists.
func (st *storeImplementation) Has(ctx context.Context, key string, options SessionOptionsInterface) (bool, error) {
	session, err := st.SessionFindByKey(ctx, key, options)
	if err != nil {
		return false, err
	}
	return session != nil, nil
}

// MergeMap merges a map into an existing session value.
func (st *storeImplementation) MergeMap(ctx context.Context, key string, value map[string]any, seconds int64, options SessionOptionsInterface) error {
	session, err := st.SessionFindByKey(ctx, key, options)
	if err != nil {
		return err
	}

	var existingValue map[string]any
	if session != nil {
		jsonValue, err := st.decryptValue(session.GetValue())
		if err != nil {
			return err
		}
		if jsonValue != "" {
			if err := json.Unmarshal([]byte(jsonValue), &existingValue); err != nil {
				return err
			}
		}
	}

	if existingValue == nil {
		existingValue = make(map[string]any)
	}

	for k, v := range value {
		existingValue[k] = v
	}

	return st.SetMap(ctx, key, existingValue, seconds, options)
}

// Set stores a value in the session with a TTL.
func (st *storeImplementation) Set(ctx context.Context, key string, value string, seconds int64, options SessionOptionsInterface) error {
	session, err := st.SessionFindByKey(ctx, key, options)
	if err != nil {
		return err
	}

	encryptedValue, err := st.encryptValue(value)
	if err != nil {
		return err
	}

	if session == nil {
		newSession := NewSession()
		newSession.SetKey(key)
		newSession.SetValue(encryptedValue)
		newSession.SetExpiresAt(carbon.Now(carbon.UTC).AddSeconds(cast.ToInt(seconds)).ToDateTimeString(carbon.UTC))
		return st.SessionCreate(ctx, newSession)
	}

	session.SetValue(encryptedValue)
	session.SetExpiresAt(carbon.Now(carbon.UTC).AddSeconds(cast.ToInt(seconds)).ToDateTimeString(carbon.UTC))
	return st.SessionUpdate(ctx, session)
}

// SetAny stores an interface{} value as JSON in the session.
func (st *storeImplementation) SetAny(ctx context.Context, key string, value any, seconds int64, options SessionOptionsInterface) error {
	jsonValue, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return st.Set(ctx, key, string(jsonValue), seconds, options)
}

// SetMap stores a map[string]any value as JSON in the session.
func (st *storeImplementation) SetMap(ctx context.Context, key string, value map[string]any, seconds int64, options SessionOptionsInterface) error {
	jsonValue, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return st.Set(ctx, key, string(jsonValue), seconds, options)
}

// == NEW API =================================================================

// SessionCount counts sessions based on a query.
func (st *storeImplementation) SessionCount(ctx context.Context, query SessionQueryInterface) (int64, error) {
	if ctx == nil {
		return 0, errors.New("ctx is nil")
	}

	q := st.buildQuery(query)

	var count int64
	err := q.Table(st.sessionTableName).Count(&count)
	return count, err
}

// SessionCreate creates a new session.
func (st *storeImplementation) SessionCreate(ctx context.Context, s SessionInterface) error {
	if s == nil {
		return errors.New("session store: session cannot be nil")
	}
	if s.GetKey() == "" {
		return errors.New("session store: key cannot be empty")
	}
	if s.GetExpiresAt() == "" {
		return errors.New("session store: expires_at cannot be empty")
	}
	if s.GetCreatedAt() == "" {
		s.SetCreatedAt(carbon.Now(carbon.UTC).ToDateTimeString())
	}
	if s.GetUpdatedAt() == "" {
		s.SetUpdatedAt(carbon.Now(carbon.UTC).ToDateTimeString())
	}
	if s.GetSoftDeletedAt() == "" {
		s.SetSoftDeletedAt(MAX_DATETIME)
	}

	encryptedValue, err := st.encryptValue(s.GetValue())
	if err != nil {
		return err
	}

	row := map[string]any{
		COLUMN_ID:              s.GetID(),
		COLUMN_SESSION_KEY:     s.GetKey(),
		COLUMN_USER_ID:         s.GetUserID(),
		COLUMN_IP_ADDRESS:      s.GetIPAddress(),
		COLUMN_USER_AGENT:      s.GetUserAgent(),
		COLUMN_SESSION_VALUE:   encryptedValue,
		COLUMN_EXPIRES_AT:      s.GetExpiresAtCarbon().StdTime(),
		COLUMN_CREATED_AT:      s.GetCreatedAtCarbon().StdTime(),
		COLUMN_UPDATED_AT:      s.GetUpdatedAtCarbon().StdTime(),
		COLUMN_SOFT_DELETED_AT: s.GetSoftDeletedAtCarbon().StdTime(),
	}

	return st.db.Query().Table(st.sessionTableName).Create(row)
}

// SessionDelete permanently deletes a session.
func (st *storeImplementation) SessionDelete(ctx context.Context, s SessionInterface) error {
	if ctx == nil {
		return errors.New("ctx is nil")
	}
	if s == nil {
		return errors.New("session is nil")
	}
	return st.SessionDeleteByID(ctx, s.GetID())
}

// SessionDeleteByID permanently deletes a session by ID.
func (st *storeImplementation) SessionDeleteByID(ctx context.Context, id string) error {
	if ctx == nil {
		return errors.New("ctx is nil")
	}
	if id == "" {
		return errors.New("session id is empty")
	}

	_, err := st.db.Query().
		Table(st.sessionTableName).
		Where(COLUMN_ID+" = ?", id).
		Delete()
	return err
}

// SessionExtend extends a session's expiry by the given seconds.
func (st *storeImplementation) SessionExtend(ctx context.Context, s SessionInterface, seconds int64) error {
	if s == nil {
		return errors.New("session is nil")
	}
	expiresAt := carbon.Now(carbon.UTC).AddSeconds(cast.ToInt(seconds)).ToDateTimeString(carbon.UTC)
	s.SetExpiresAt(expiresAt)
	return st.SessionUpdate(ctx, s)
}

// SessionFindByID finds an active (non-expired) session by ID.
func (st *storeImplementation) SessionFindByID(ctx context.Context, sessionID string, options ...SessionOptionsInterface) (SessionInterface, error) {
	if sessionID == "" {
		return nil, errors.New("session store: session id is required")
	}

	o := lo.FirstOr(options, NewSessionOptions())
	query := SessionQuery().
		SetID(sessionID).
		SetExpiresAtGte(carbon.Now(carbon.UTC).ToDateTimeString(carbon.UTC)).
		SetLimit(1)

	if o.HasIPAddress() {
		query.SetUserIpAddress(o.GetIPAddress())
	}
	if o.HasUserAgent() {
		query.SetUserAgent(o.GetUserAgent())
	}
	if o.HasUserID() {
		query.SetUserID(o.GetUserID())
	}

	list, err := st.SessionList(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(list) > 0 {
		return list[0], nil
	}
	return nil, nil
}

// SessionFindByKey finds an active (non-expired) session by key.
func (st *storeImplementation) SessionFindByKey(ctx context.Context, sessionKey string, options ...SessionOptionsInterface) (SessionInterface, error) {
	if sessionKey == "" {
		return nil, errors.New("session store: session key is required")
	}

	o := lo.FirstOr(options, NewSessionOptions())
	query := SessionQuery().
		SetKey(sessionKey).
		SetExpiresAtGte(carbon.Now(carbon.UTC).ToDateTimeString(carbon.UTC)).
		SetLimit(1)

	if o.HasIPAddress() {
		query.SetUserIpAddress(o.GetIPAddress())
	}
	if o.HasUserAgent() {
		query.SetUserAgent(o.GetUserAgent())
	}
	if o.HasUserID() {
		query.SetUserID(o.GetUserID())
	}

	list, err := st.SessionList(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(list) > 0 {
		return list[0], nil
	}
	return nil, nil
}

// SessionList lists sessions based on a query.
func (st *storeImplementation) SessionList(ctx context.Context, query SessionQueryInterface) ([]SessionInterface, error) {
	if ctx == nil {
		return nil, errors.New("ctx is nil")
	}

	type sessionRow struct {
		ID            string    `db:"id"`
		Key           string    `db:"session_key"`
		UserID        string    `db:"user_id"`
		IPAddress     string    `db:"ip_address"`
		UserAgent     string    `db:"user_agent"`
		Value         string    `db:"session_value"`
		ExpiresAt     time.Time `db:"expires_at"`
		CreatedAt     time.Time `db:"created_at"`
		UpdatedAt     time.Time `db:"updated_at"`
		SoftDeletedAt time.Time `db:"soft_deleted_at"`
	}

	q := st.buildQuery(query)

	var rows []sessionRow
	if err := q.Table(st.sessionTableName).Get(&rows); err != nil {
		return []SessionInterface{}, err
	}

	list := make([]SessionInterface, 0, len(rows))
	for _, r := range rows {
		decryptedValue, err := st.decryptValue(r.Value)
		if err != nil {
			return []SessionInterface{}, err
		}

		s := &sessionImplementation{}
		s.SetID(r.ID)
		s.SetKey(r.Key)
		s.SetUserID(r.UserID)
		s.SetIPAddress(r.IPAddress)
		s.SetUserAgent(r.UserAgent)
		s.SetValue(decryptedValue)
		s.ExpiresAtField = r.ExpiresAt
		s.CreatedAtField.CreatedAt = r.CreatedAt
		s.UpdatedAtField.UpdatedAt = r.UpdatedAt
		s.SoftDeletedAt = r.SoftDeletedAt
		list = append(list, s)
	}

	return list, nil
}

// SessionSoftDelete soft-deletes a session.
func (st *storeImplementation) SessionSoftDelete(ctx context.Context, s SessionInterface) error {
	if ctx == nil {
		return errors.New("ctx is nil")
	}
	if s == nil {
		return errors.New("session is nil")
	}

	s.SetSoftDeletedAt(carbon.Now(carbon.UTC).ToDateTimeString(carbon.UTC))

	row := map[string]any{
		COLUMN_SOFT_DELETED_AT: s.GetSoftDeletedAtCarbon().StdTime(),
		COLUMN_UPDATED_AT:      carbon.Now(carbon.UTC).StdTime(),
	}

	_, err := st.db.Query().Table(st.sessionTableName).Where(COLUMN_ID+" = ?", s.GetID()).Update(row)
	return err
}

// SessionSoftDeleteByID soft-deletes a session by ID.
func (st *storeImplementation) SessionSoftDeleteByID(ctx context.Context, id string) error {
	if ctx == nil {
		return errors.New("ctx is nil")
	}
	if id == "" {
		return errors.New("session id is empty")
	}

	softDeletedAt := carbon.Now(carbon.UTC).ToDateTimeString(carbon.UTC)

	row := map[string]any{
		COLUMN_SOFT_DELETED_AT: carbon.Parse(softDeletedAt, carbon.UTC).StdTime(),
		COLUMN_UPDATED_AT:      carbon.Now(carbon.UTC).StdTime(),
	}

	_, err := st.db.Query().Table(st.sessionTableName).Where(COLUMN_ID+" = ?", id).Update(row)
	return err
}

// SessionUpdate updates a session.
func (st *storeImplementation) SessionUpdate(ctx context.Context, s SessionInterface) error {
	if s == nil {
		return errors.New("session is nil")
	}

	encryptedValue, err := st.encryptValue(s.GetValue())
	if err != nil {
		return err
	}

	row := map[string]any{
		COLUMN_SESSION_VALUE: encryptedValue,
		COLUMN_UPDATED_AT:    carbon.Now(carbon.UTC).StdTime(),
		COLUMN_EXPIRES_AT:    s.GetExpiresAtCarbon().StdTime(),
	}

	_, err = st.db.Query().Table(st.sessionTableName).Where(COLUMN_ID+" = ?", s.GetID()).Update(row)
	return err
}

// == QUERY BUILDER ==========================================================

// buildQuery builds a neat query from the session query interface.
func (st *storeImplementation) buildQuery(query SessionQueryInterface) contractsorm.Query {
	// Use Model() to enable neat's automatic soft delete handling via SoftDeletesMaxDate
	q := st.db.Query().Model(&sessionImplementation{})

	if query == nil {
		return q
	}

	if query.HasID() && query.ID() != "" {
		q = q.Where(COLUMN_ID+" = ?", query.ID())
	}

	if query.HasIDIn() && len(query.IDIn()) > 0 {
		q = q.Where(COLUMN_ID+" IN ?", query.IDIn())
	}

	if query.HasKey() && query.Key() != "" {
		q = q.Where(COLUMN_SESSION_KEY+" = ?", query.Key())
	}

	if query.HasUserID() && query.UserID() != "" {
		q = q.Where(COLUMN_USER_ID+" = ?", query.UserID())
	}

	if query.HasUserIpAddress() && query.UserIpAddress() != "" {
		q = q.Where(COLUMN_IP_ADDRESS+" = ?", query.UserIpAddress())
	}

	if query.HasUserAgent() && query.UserAgent() != "" {
		q = q.Where(COLUMN_USER_AGENT+" = ?", query.UserAgent())
	}

	if query.HasExpiresAtGte() && query.ExpiresAtGte() != "" {
		q = q.Where(COLUMN_EXPIRES_AT+" >= ?", query.ExpiresAtGte())
	}

	if query.HasExpiresAtLte() && query.ExpiresAtLte() != "" {
		q = q.Where(COLUMN_EXPIRES_AT+" <= ?", query.ExpiresAtLte())
	}

	if query.HasCreatedAtGte() && query.CreatedAtGte() != "" {
		q = q.Where(COLUMN_CREATED_AT+" >= ?", query.CreatedAtGte())
	}

	if query.HasCreatedAtLte() && query.CreatedAtLte() != "" {
		q = q.Where(COLUMN_CREATED_AT+" <= ?", query.CreatedAtLte())
	}

	if query.HasLimit() && query.Limit() > 0 {
		q = q.Limit(query.Limit())
	}

	if query.HasOffset() && query.Offset() > 0 {
		q = q.Offset(query.Offset())
	}

	if query.HasOrderBy() && query.OrderBy() != "" {
		direction := lo.CoalesceOrEmpty(query.SortOrder(), "DESC")
		q = q.OrderBy(query.OrderBy() + " " + direction)
	}

	// Handle soft delete filtering via neat's automatic handling (SoftDeletesMaxDate)
	if query.HasSoftDeletedIncluded() && query.SoftDeletedIncluded() {
		q = q.WithSoftDeleted()
	}

	return q
}

// == ENCRYPTION ==============================================================

// encryptValue encrypts a value if encryption is enabled.
func (st *storeImplementation) encryptValue(value string) (string, error) {
	if st.encryptor == nil {
		return value, nil
	}
	return st.encryptor.encrypt(value)
}

// decryptValue decrypts a value if encryption is enabled.
func (st *storeImplementation) decryptValue(value string) (string, error) {
	if st.encryptor == nil {
		return value, nil
	}
	return st.encryptor.decrypt(value)
}
