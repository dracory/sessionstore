package sessionstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/dracory/neat"
	contractsorm "github.com/dracory/neat/contracts/database/orm"
	contractsschema "github.com/dracory/neat/contracts/database/schema"
	"github.com/dromara/carbon/v2"
	"github.com/samber/lo"
	"github.com/spf13/cast"
)

// == INTERFACE ===============================================================

var _ StoreInterface = (*storeImplementation)(nil)

// == TYPE ====================================================================

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

	now := time.Now().UTC()
	_, err := st.db.Query().
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

// Has checks if a session with the given key exists.
func (st *storeImplementation) Has(ctx context.Context, sessionKey string, options SessionOptionsInterface) (bool, error) {
	if sessionKey == "" {
		return false, errors.New("session store: session key is required")
	}

	query := SessionQuery().
		SetKey(sessionKey).
		SetExpiresAtGte(carbon.Now(carbon.UTC).ToDateTimeString(carbon.UTC)).
		SetLimit(1)

	if options.HasIPAddress() {
		query.SetUserIpAddress(options.GetIPAddress())
	}
	if options.HasUserAgent() {
		query.SetUserAgent(options.GetUserAgent())
	}
	if options.HasUserID() {
		query.SetUserID(options.GetUserID())
	}

	count, err := st.SessionCount(ctx, query)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// MergeMap merges the given map into the existing session value map.
func (st *storeImplementation) MergeMap(ctx context.Context, key string, mergeMap map[string]any, seconds int64, options SessionOptionsInterface) error {
	currentMap, err := st.GetMap(ctx, key, nil, options)
	if err != nil {
		return err
	}
	if currentMap == nil {
		return errors.New("session store: nil map found")
	}
	for k, v := range mergeMap {
		currentMap[k] = v
	}
	return st.SetMap(ctx, key, currentMap, seconds, options)
}

// Set sets a session key/value pair, creating or updating as needed.
func (st *storeImplementation) Set(ctx context.Context, sessionKey string, value string, seconds int64, options SessionOptionsInterface) error {
	session, err := st.SessionFindByKey(ctx, sessionKey, options)
	if err != nil {
		return err
	}

	expiresAt := carbon.Now(carbon.UTC).AddSeconds(cast.ToInt(seconds)).ToDateTimeString(carbon.UTC)

	if session == nil {
		newSession := NewSession().
			SetKey(sessionKey).
			SetValue(value).
			SetUserID(options.GetUserID()).
			SetUserAgent(options.GetUserAgent()).
			SetIPAddress(options.GetIPAddress()).
			SetExpiresAt(expiresAt)
		return st.SessionCreate(ctx, newSession)
	}

	session.SetValue(value)
	session.SetExpiresAt(expiresAt)
	session.SetUpdatedAt(carbon.Now(carbon.UTC).ToDateTimeString(carbon.UTC))
	return st.SessionUpdate(ctx, session)
}

// SetAny sets a session value by serializing the supplied interface to JSON.
func (st *storeImplementation) SetAny(ctx context.Context, key string, value any, seconds int64, options SessionOptionsInterface) error {
	jsonValue, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return st.Set(ctx, key, string(jsonValue), seconds, options)
}

// SetMap sets a session value by serializing the supplied map to JSON.
func (st *storeImplementation) SetMap(ctx context.Context, key string, value map[string]any, seconds int64, options SessionOptionsInterface) error {
	jsonValue, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return st.Set(ctx, key, string(jsonValue), seconds, options)
}

// == NEW API =================================================================

// SessionCount returns the count of sessions matching the query.
func (st *storeImplementation) SessionCount(ctx context.Context, query SessionQueryInterface) (int64, error) {
	if query == nil {
		return -1, errors.New("session store: session query is nil")
	}
	if err := query.Validate(); err != nil {
		return -1, err
	}

	q := st.buildQuery(query)

	var count int64
	if err := q.Table(st.sessionTableName).Count(&count); err != nil {
		return -1, err
	}
	return count, nil
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

// SessionList returns a list of sessions matching the query.
func (st *storeImplementation) SessionList(ctx context.Context, query SessionQueryInterface) ([]SessionInterface, error) {
	if query == nil {
		return []SessionInterface{}, errors.New("session store: session query is nil")
	}
	if err := query.Validate(); err != nil {
		return []SessionInterface{}, err
	}

	type sessionRow struct {
		ID            string     `db:"id"`
		Key           string     `db:"session_key"`
		UserID        string     `db:"user_id"`
		IPAddress     string     `db:"ip_address"`
		UserAgent     string     `db:"user_agent"`
		Value         string     `db:"session_value"`
		ExpiresAt     time.Time  `db:"expires_at"`
		CreatedAt     time.Time  `db:"created_at"`
		UpdatedAt     time.Time  `db:"updated_at"`
		SoftDeletedAt *time.Time `db:"soft_deleted_at"`
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
		if r.SoftDeletedAt != nil {
			s.DeletedAt = sql.NullTime{Time: *r.SoftDeletedAt, Valid: true}
		}
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
	return st.SessionUpdate(ctx, s)
}

// SessionSoftDeleteByID soft-deletes a session by ID.
func (st *storeImplementation) SessionSoftDeleteByID(ctx context.Context, id string) error {
	s, err := st.SessionFindByID(ctx, id)
	if err != nil {
		return err
	}
	return st.SessionSoftDelete(ctx, s)
}

// SessionUpdate updates an existing session.
func (st *storeImplementation) SessionUpdate(ctx context.Context, s SessionInterface) error {
	if s == nil {
		return errors.New("session store: session cannot be nil")
	}
	if st.db == nil {
		return errors.New("session store: db cannot be nil")
	}

	s.SetUpdatedAt(carbon.Now(carbon.UTC).ToDateTimeString(carbon.UTC))

	encryptedValue, err := st.encryptValue(s.GetValue())
	if err != nil {
		return err
	}

	updateData := map[string]any{
		COLUMN_SESSION_KEY:     s.GetKey(),
		COLUMN_USER_ID:         s.GetUserID(),
		COLUMN_IP_ADDRESS:      s.GetIPAddress(),
		COLUMN_USER_AGENT:      s.GetUserAgent(),
		COLUMN_SESSION_VALUE:   encryptedValue,
		COLUMN_EXPIRES_AT:      s.GetExpiresAtCarbon().StdTime(),
		COLUMN_UPDATED_AT:      s.GetUpdatedAtCarbon().StdTime(),
		COLUMN_SOFT_DELETED_AT: s.GetSoftDeletedAtCarbon().StdTime(),
	}

	_, err = st.db.Query().
		Table(st.sessionTableName).
		Where(COLUMN_ID+" = ?", s.GetID()).
		Update(updateData)
	return err
}

// == HELPERS =================================================================

// buildQuery converts a SessionQueryInterface into a neat ORM query.
// Datetime values are passed as time.Time so the ORM driver formats them
// in the correct dialect-specific format (e.g. RFC3339 for SQLite).
func (st *storeImplementation) buildQuery(options SessionQueryInterface) contractsorm.Query {
	q := st.db.Query()

	if options.HasCreatedAtGte() {
		q = q.Where(COLUMN_CREATED_AT+" >= ?", carbon.Parse(options.CreatedAtGte(), carbon.UTC).StdTime())
	}
	if options.HasCreatedAtLte() {
		q = q.Where(COLUMN_CREATED_AT+" <= ?", carbon.Parse(options.CreatedAtLte(), carbon.UTC).StdTime())
	}

	if options.HasExpiresAtGte() {
		q = q.Where(COLUMN_EXPIRES_AT+" >= ?", carbon.Parse(options.ExpiresAtGte(), carbon.UTC).StdTime())
	}
	if options.HasExpiresAtLte() {
		q = q.Where(COLUMN_EXPIRES_AT+" <= ?", carbon.Parse(options.ExpiresAtLte(), carbon.UTC).StdTime())
	}

	if options.HasID() {
		q = q.Where(COLUMN_ID+" = ?", options.ID())
	}

	if options.HasIDIn() {
		ids := make([]any, len(options.IDIn()))
		for i, id := range options.IDIn() {
			ids[i] = id
		}
		q = q.WhereIn(COLUMN_ID, ids)
	}

	if options.HasKey() {
		q = q.Where(COLUMN_SESSION_KEY+" = ?", options.Key())
	}

	if options.HasUserAgent() {
		q = q.Where(COLUMN_USER_AGENT+" = ?", options.UserAgent())
	}

	if options.HasUserID() {
		q = q.Where(COLUMN_USER_ID+" = ?", options.UserID())
	}

	if options.HasUserIpAddress() {
		q = q.Where(COLUMN_IP_ADDRESS+" = ?", options.UserIpAddress())
	}

	if options.HasLimit() {
		q = q.Limit(options.Limit())
	}

	if options.HasOffset() {
		q = q.Offset(options.Offset())
	}

	if options.HasOrderBy() && options.OrderBy() != "" {
		direction := "desc"
		if options.HasSortOrder() && strings.EqualFold(options.SortOrder(), "asc") {
			direction = "asc"
		}
		q = q.OrderBy(options.OrderBy(), direction)
	}

	if !options.SoftDeletedIncluded() {
		// Exclude soft-deleted rows: soft_deleted_at must be in the future.
		// Pass time.Time so the driver serialises it in the correct format.
		q = q.Where(COLUMN_SOFT_DELETED_AT+" > ?", time.Now().UTC())
	}

	return q
}

// encryptValue encrypts the session value if an encryptor is configured.
func (st *storeImplementation) encryptValue(value string) (string, error) {
	if st == nil || st.encryptor == nil {
		return value, nil
	}
	return st.encryptor.encrypt(value)
}

// decryptValue decrypts the session value if an encryptor is configured.
func (st *storeImplementation) decryptValue(value string) (string, error) {
	if st == nil || st.encryptor == nil {
		return value, nil
	}
	return st.encryptor.decrypt(value)
}
