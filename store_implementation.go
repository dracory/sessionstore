package sessionstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/dracory/database"
	"github.com/dracory/sb"
	"github.com/dromara/carbon/v2"
	"github.com/georgysavva/scany/sqlscan"
	"github.com/spf13/cast"
)

// == INTERFACE ===============================================================

var _ StoreInterface = (*storeImplementation)(nil) // verify it extends the store interface

// == TYPE ====================================================================

// Store defines a session store
type storeImplementation struct {
	sessionTableName   string
	db                 *sql.DB
	dbDriverName       string
	timeoutSeconds     int64
	automigrateEnabled bool
	debugEnabled       bool
	sqlLogger          *slog.Logger
	encryptor          *sessionEncryptor
}

// PUBLIC METHODS ============================================================

// AutoMigrate creates the session table if it does not exist
//
// Parameters:
//   - ctx - the context
//
// Returns:
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) AutoMigrate(ctx context.Context) error {
	sqlStr := st.SQLCreateTable()

	if sqlStr == "" {
		return errors.New("session store: table create sql is empty")
	}

	if st.db == nil {
		return errors.New("session store: database is nil")
	}

	_, err := database.Execute(database.Context(ctx, st.db), sqlStr)

	if err != nil {
		return err
	}

	return nil
}

// EnableDebug enables the debug mode
//
// # If debug mode is enabled, it will print the SQL statements to the logger
//
// Parameters:
//   - debug - true to enable, false to disable
//
// Returns:
//   - void
func (st *storeImplementation) EnableDebug(debug bool) {
	st.debugEnabled = debug
}

// GetDB returns the database connection
func (st *storeImplementation) GetDB() *sql.DB {
	return st.db
}

// SessionExpiryGoroutine this is a goroutine that deletes expired sessions
// honoring the provided context.
// It runs periodically (every minute) and deletes any sessions that have expired.
//
// This is a goroutine that runs periodically (every minute) and deletes
// any sessions that have expired
//
// Parameters:
//   - ctx - the context
//
// Returns:
//   - error - nil if successful, otherwise an error
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
		log.Println("Cleaning expired sessions...")
	}

	sqlStr, sqlParams, err := goqu.Dialect(st.dbDriverName).
		From(st.sessionTableName).
		Where(goqu.C(COLUMN_EXPIRES_AT).Lt(time.Now())).
		Delete().
		Prepared(true).
		ToSQL()

	if err != nil {
		return err
	}

	st.logSql("delete", sqlStr, sqlParams)

	if ctx.Err() != nil {
		return ctx.Err()
	}

	_, err = database.Execute(database.Context(ctx, st.db), sqlStr, sqlParams...)

	if err != nil {
		if errors.Is(err, context.Canceled) {
			return context.Canceled
		}
		if err == sql.ErrNoRows {
			// Looks like this is now outdated for sqlscan
			return nil
		}

		if sqlscan.NotFound(err) {
			return nil
		}

		log.Println("Session Store. ExpireSessionGoroutine. Error: ", err)
		return nil
	}

	return nil
}

// Extend extends the session expiry time with the given seconds.
//
// Parameters:
//   - ctx - the context
//   - sessionKey - the session key
//   - seconds - the number of seconds to extend the session by
//   - options - the session options
//
// Returns:
//   - error - nil if successful, otherwise an error
func (store *storeImplementation) Extend(ctx context.Context, sessionKey string, seconds int64, options SessionOptionsInterface) error {
	session, errFindByKey := store.FindByKey(ctx, sessionKey, options)

	if errFindByKey != nil {
		return errFindByKey
	}

	if session == nil {
		return errors.New("session not found")
	}

	expiresAt := carbon.Now(carbon.UTC).AddSeconds(cast.ToInt(seconds)).ToDateTimeString(carbon.UTC)

	session.SetExpiresAt(expiresAt)

	err := store.SessionUpdate(ctx, session)

	if err != nil {
		return err
	}

	return nil
}

// Delete deletes a session.
//
// Parameters:
//   - ctx - the context
//   - sessionKey - the session key
//   - options - the session options
//
// Returns:
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) Delete(ctx context.Context, sessionKey string, options SessionOptionsInterface) error {
	wheres := []goqu.Expression{
		goqu.C(COLUMN_SESSION_KEY).Eq(sessionKey),
	}

	if options.HasUserAgent() {
		wheres = append(wheres, goqu.C(COLUMN_USER_AGENT).Eq(options.GetUserAgent()))
	}

	if options.HasUserID() {
		wheres = append(wheres, goqu.C(COLUMN_USER_ID).Eq(options.GetUserID()))
	}

	if options.HasIPAddress() {
		wheres = append(wheres, goqu.C(COLUMN_IP_ADDRESS).Eq(options.GetIPAddress()))
	}

	sqlStr, sqlParams, err := goqu.Dialect(st.dbDriverName).
		From(st.sessionTableName).
		Where(wheres...).
		Delete().
		Prepared(true).
		ToSQL()

	if err != nil {
		return err
	}

	if st.debugEnabled {
		log.Println(sqlStr)
	}

	_, err = st.db.Exec(sqlStr, sqlParams...)

	if err != nil {
		if err == sql.ErrNoRows {
			// Looks like this is now outdated for sqlscan
			return nil
		}

		if sqlscan.NotFound(err) {
			return nil
		}

		return err
	}

	return nil
}

// FindByKey finds a session by key.
//
// Parameters:
//   - ctx - the context
//   - sessionKey - the session key
//   - options - the session options
//
// Returns:
//   - SessionInterface - the found session
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) FindByKey(ctx context.Context, sessionKey string, options SessionOptionsInterface) (SessionInterface, error) {
	if sessionKey == "" {
		return nil, errors.New("session store > find by key: session key is required")
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

	list, err := st.SessionList(ctx, query)

	if err != nil {
		return nil, err
	}

	if len(list) > 0 {
		return list[0], nil
	}

	return nil, nil
}

// Get is a shortcut for getting the value of a session, or a default value if not found
//
// # It is a convenience method for getting the value of a session wrapping
//
// Parameters:
//   - ctx - the context
//   - sessionKey - the session key
//   - valueDefault - the default value to return if session not found
//   - options - the session options
//
// Returns:
//   - string - the session value
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) Get(ctx context.Context, sessionKey string, valueDefault string, options SessionOptionsInterface) (string, error) {
	session, errFindByKey := st.FindByKey(ctx, sessionKey, options)

	if errFindByKey != nil {
		return "", errFindByKey
	}

	if session != nil {
		decrypted, err := st.decryptValue(session.GetValue())
		if err != nil {
			return "", err
		}
		return decrypted, nil
	}

	return valueDefault, nil
}

// GetAny attempts to parse the value as interface, use with SetAny.
//
// Parameters:
//   - ctx - the context
//   - key - the session key
//   - valueDefault - the default value to return if session not found
//   - options - the session options
//
// Returns:
//   - interface{} - the parsed value
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) GetAny(ctx context.Context, key string, valueDefault interface{}, options SessionOptionsInterface) (interface{}, error) {
	session, errFindByKey := st.FindByKey(ctx, key, options)

	if errFindByKey != nil {
		return valueDefault, errFindByKey
	}

	if session != nil {
		jsonValue, errDecrypt := st.decryptValue(session.GetValue())
		if errDecrypt != nil {
			return valueDefault, errDecrypt
		}
		var val interface{}
		jsonError := json.Unmarshal([]byte(jsonValue), &val)
		if jsonError != nil {
			return valueDefault, jsonError
		}

		return val, nil
	}

	return valueDefault, nil
}

// GetMap attempts to parse the value as map[string]any, use with SetMap.
//
// Parameters:
//   - ctx - the context
//   - key - the session key
//   - valueDefault - the default map to return if session not found
//   - options - the session options
//
// Returns:
//   - map[string]any - the parsed map
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) GetMap(ctx context.Context, key string, valueDefault map[string]any, options SessionOptionsInterface) (map[string]any, error) {
	session, errFindByKey := st.FindByKey(ctx, key, options)

	if errFindByKey != nil {
		return valueDefault, errFindByKey
	}

	if session != nil {
		jsonValue, errDecrypt := st.decryptValue(session.GetValue())
		if errDecrypt != nil {
			return valueDefault, errDecrypt
		}
		var val map[string]any
		jsonError := json.Unmarshal([]byte(jsonValue), &val)
		if jsonError != nil {
			return valueDefault, jsonError
		}

		return val, nil
	}

	return valueDefault, nil
}

// Has checks if a session with the given key exists.
//
// Parameters:
//   - ctx - the context
//   - sessionKey - the session key
//   - options - the session options
//
// Returns:
//   - bool - true if session exists, false otherwise
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) Has(ctx context.Context, sessionKey string, options SessionOptionsInterface) (bool, error) {
	if sessionKey == "" {
		return false, errors.New("session store > find by key: session key is required")
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

// MergeMap merges the given map with the existing session value map.
//
// Parameters:
//   - ctx - the context
//   - key - the session key
//   - mergeMap - the map to merge with existing session value
//   - seconds - the number of seconds to extend the session by
//   - options - the session options
//
// Returns:
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) MergeMap(ctx context.Context, key string, mergeMap map[string]any, seconds int64, options SessionOptionsInterface) error {
	currentMap, err := st.GetMap(ctx, key, nil, options)

	if err != nil {
		return err
	}

	if currentMap == nil {
		return errors.New("sessionstore. nil found")
	}

	for mapKey, mapValue := range mergeMap {
		currentMap[mapKey] = mapValue
	}

	return st.SetMap(ctx, key, currentMap, seconds, options)
}

// SessionCount returns the count of sessions matching the query.
//
// Parameters:
//   - ctx - the context
//   - options - the session query options
//
// Returns:
//   - int64 - the count of matching sessions
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) SessionCount(ctx context.Context, query SessionQueryInterface) (int64, error) {
	query.SetCountOnly(true)

	q, _, err := st.sessionSelectQuery(query)

	if err != nil {
		return -1, err
	}

	sqlStr, params, errSql := q.Prepared(true).
		Limit(1).
		Select(goqu.COUNT(goqu.Star()).As("count")).
		ToSQL()

	if errSql != nil {
		return -1, nil
	}

	if st.debugEnabled {
		log.Println(sqlStr)
	}

	mapped, err := database.SelectToMapString(database.Context(ctx, st.db), sqlStr, params...)

	if err != nil {
		return -1, err
	}

	if len(mapped) < 1 {
		return -1, nil
	}

	countStr := mapped[0]["count"]

	i, err := strconv.ParseInt(countStr, 10, 64)

	if err != nil {
		return -1, err

	}

	return i, nil
}

// SessionCreate creates a new session.
//
// Parameters:
//   - ctx - the context
//   - session - the session to create
//
// Returns:
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) SessionCreate(ctx context.Context, session SessionInterface) error {
	if session == nil {
		return errors.New("sessionstore > session create. session cannot be nil")
	}

	if session.GetKey() == "" {
		return errors.New("sessionstore > session create. key cannot be empty")
	}

	if session.GetExpiresAt() == "" {
		return errors.New("sessionstore > session create. expires at cannot be empty")
	}

	if session.GetCreatedAt() == "" {
		session.SetCreatedAt(carbon.Now(carbon.UTC).ToDateTimeString())
	}

	if session.GetUpdatedAt() == "" {
		session.SetUpdatedAt(carbon.Now(carbon.UTC).ToDateTimeString())
	}

	if session.GetSoftDeletedAt() == "" {
		session.SetSoftDeletedAt(sb.MAX_DATETIME)
	}

	originalData := session.Data()
	data := make(map[string]interface{}, len(originalData))

	for key, value := range originalData {
		if key == COLUMN_SESSION_VALUE {
			encryptedValue, err := st.encryptValue(value)
			if err != nil {
				return err
			}
			data[key] = encryptedValue
			continue
		}
		data[key] = value
	}

	sqlStr, sqlParams, sqlErr := goqu.Dialect(st.dbDriverName).
		Insert(st.sessionTableName).
		Prepared(true).
		Rows(data).
		ToSQL()

	if sqlErr != nil {
		return sqlErr
	}

	st.logSql("create", sqlStr, sqlParams)

	_, err := st.db.Exec(sqlStr, sqlParams...)

	if err != nil {
		return err
	}

	session.MarkAsNotDirty()

	return nil
}

// SessionDelete deletes a session.
//
// Parameters:
//   - ctx - the context
//   - session - the session to delete
//
// Returns:
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) SessionDelete(ctx context.Context, session SessionInterface) error {
	if ctx == nil {
		return errors.New("ctx is nil")
	}

	if session == nil {
		return errors.New("session is nil")
	}

	return st.SessionDeleteByID(ctx, session.GetID())
}

// SessionDeleteByID deletes a session by id.
//
// Parameters:
//   - ctx - the context
//   - id - the session id
//
// Returns:
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) SessionDeleteByID(ctx context.Context, id string) error {
	if ctx == nil {
		return errors.New("ctx is nil")
	}

	if id == "" {
		return errors.New("session id is empty")
	}

	sqlStr, params, errSql := goqu.Dialect(st.dbDriverName).
		Delete(st.sessionTableName).
		Prepared(true).
		Where(goqu.C(COLUMN_ID).Eq(id)).
		ToSQL()

	if errSql != nil {
		return errSql
	}

	st.logSql("delete", sqlStr, params...)

	_, err := st.db.Exec(sqlStr, params...)

	return err
}

// SessionDeleteByKey deletes a session by key.
//
// Parameters:
//   - sessionKey - the session key
//
// Returns:
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) SessionDeleteByKey(sessionKey string) error {
	if sessionKey == "" {
		return errors.New("session id is empty")
	}

	sqlStr, params, errSql := goqu.Dialect(st.dbDriverName).
		Delete(st.sessionTableName).
		Prepared(true).
		Where(goqu.C(COLUMN_SESSION_KEY).Eq(sessionKey)).
		ToSQL()

	if errSql != nil {
		return errSql
	}

	st.logSql("delete", sqlStr, params...)

	_, err := st.db.Exec(sqlStr, params...)

	return err
}

// SessionExtend extends a session's expiry time by the given seconds.
//
// Parameters:
//   - ctx - the context
//   - session - the session to extend
//   - seconds - the number of seconds to extend the session by
//
// Returns:
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) SessionExtend(ctx context.Context, session SessionInterface, seconds int64) error {
	if session == nil {
		return errors.New("session is nil")
	}

	expiresAt := carbon.Now(carbon.UTC).AddSeconds(cast.ToInt(seconds)).ToDateTimeString(carbon.UTC)

	session.SetExpiresAt(expiresAt)

	return st.SessionUpdate(ctx, session)
}

// SessionFindByID finds a session by id.
//
// Parameters:
//   - ctx - the context
//   - sessionID - the session id
//
// Returns:
//   - SessionInterface - the found session
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) SessionFindByID(ctx context.Context, sessionID string) (SessionInterface, error) {
	if sessionID == "" {
		return nil, errors.New("session store > find by id: session id is required")
	}

	query := SessionQuery().
		SetID(sessionID).
		SetExpiresAtGte(carbon.Now(carbon.UTC).ToDateTimeString(carbon.UTC)).
		SetLimit(1)

	list, err := st.SessionList(ctx, query)

	if err != nil {
		return nil, err
	}

	if len(list) > 0 {
		return list[0], nil
	}

	return nil, nil
}

// SessionFindByKey finds a session by key.
//
// Parameters:
//   - ctx - the context
//   - sessionKey - the session key
//
// Returns:
//   - SessionInterface - the found session
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) SessionFindByKey(ctx context.Context, sessionKey string) (SessionInterface, error) {
	if sessionKey == "" {
		return nil, errors.New("session store > find by key: session key is required")
	}

	query := SessionQuery().
		SetKey(sessionKey).
		SetExpiresAtGte(carbon.Now(carbon.UTC).ToDateTimeString(carbon.UTC)).
		SetLimit(1)

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
//
// Parameters:
//   - ctx - the context
//   - query - the session query options
//
// Returns:
//   - []SessionInterface - list of matching sessions
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) SessionList(ctx context.Context, query SessionQueryInterface) ([]SessionInterface, error) {
	if query == nil {
		return []SessionInterface{}, errors.New("at session list > session query is nil")
	}

	q, columns, err := st.sessionSelectQuery(query)

	if err != nil {
		return []SessionInterface{}, err
	}

	sqlStr, sqlParams, errSql := q.Prepared(true).Select(columns...).ToSQL()

	if errSql != nil {
		return []SessionInterface{}, nil
	}

	st.logSql("list", sqlStr, sqlParams...)

	if st.db == nil {
		return []SessionInterface{}, errors.New("userstore: database is nil")
	}

	modelMaps, err := database.SelectToMapString(database.Context(ctx, st.db), sqlStr, sqlParams...)

	if err != nil {
		return []SessionInterface{}, err
	}

	list := make([]SessionInterface, 0, len(modelMaps))

	for _, modelMap := range modelMaps {
		decryptedValue, err := st.decryptValue(modelMap[COLUMN_SESSION_VALUE])
		if err != nil {
			return []SessionInterface{}, err
		}

		modelMap[COLUMN_SESSION_VALUE] = decryptedValue

		model := NewSessionFromExistingData(modelMap)
		list = append(list, model)
	}

	return list, nil
}

// SessionSoftDelete soft deletes a session.
//
// Parameters:
//   - ctx - the context
//   - session - the session to soft delete
//
// Returns:
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) SessionSoftDelete(ctx context.Context, session SessionInterface) error {
	if ctx == nil {
		return errors.New("ctx is nil")
	}

	if session == nil {
		return errors.New("session is nil")
	}

	session.SetSoftDeletedAt(carbon.Now(carbon.UTC).ToDateTimeString(carbon.UTC))

	return st.SessionUpdate(ctx, session)
}

// SessionSoftDeleteByID soft deletes a session by id.
//
// Parameters:
//   - ctx - the context
//   - id - the session id
//
// Returns:
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) SessionSoftDeleteByID(ctx context.Context, id string) error {
	session, err := st.SessionFindByID(ctx, id)

	if err != nil {
		return err
	}

	return st.SessionSoftDelete(ctx, session)
}

// SessionUpdate updates a session.
//
// Parameters:
//   - ctx - the context
//   - session - the session to update
//
// Returns:
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) SessionUpdate(ctx context.Context, session SessionInterface) error {
	if session == nil {
		return errors.New("sessionstore > session update. session cannot be nil")
	}

	if st.db == nil {
		return errors.New("sessionstore > session update. db cannot be nil")
	}

	session.SetUpdatedAt(carbon.Now(carbon.UTC).ToDateTimeString(carbon.UTC))

	dataChanged := session.DataChanged()

	if len(dataChanged) == 0 {
		return nil
	}

	delete(dataChanged, COLUMN_ID) // ID cannot be updated

	updateData := make(map[string]interface{}, len(dataChanged))

	for key, value := range dataChanged {
		if key == COLUMN_SESSION_VALUE {
			encryptedValue, err := st.encryptValue(value)
			if err != nil {
				return err
			}
			updateData[key] = encryptedValue
			continue
		}
		updateData[key] = value
	}

	sqlStr, sqlParams, sqlErr := goqu.Dialect(st.dbDriverName).
		Update(st.sessionTableName).
		Prepared(true).
		Where(goqu.C(COLUMN_SESSION_KEY).Eq(session.GetKey())).
		Where(goqu.C(COLUMN_ID).Eq(session.GetID())).
		Set(updateData).
		ToSQL()

	if sqlErr != nil {
		return sqlErr
	}

	st.logSql("update", sqlStr, sqlParams...)

	_, err := database.Execute(database.Context(ctx, st.db), sqlStr, sqlParams...)

	if err != nil {
		return err
	}

	return nil
}

// Set sets a session value.
//
// Parameters:
//   - ctx - the context
//   - sessionKey - the session key
//   - value - the value to set
//   - seconds - the number of seconds until session expires
//   - options - the session options
//
// Returns:
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) Set(ctx context.Context, sessionKey string, value string, seconds int64, options SessionOptionsInterface) error {
	session, errFindByKey := st.FindByKey(ctx, sessionKey, options)

	if errFindByKey != nil {
		return errFindByKey
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
	} else {
		session.SetValue(value)
		session.SetExpiresAt(expiresAt)
		session.SetUpdatedAt(carbon.Now(carbon.UTC).ToDateTimeString(carbon.UTC))

		return st.SessionUpdate(ctx, session)
	}
}

// SetAny sets a session value by serializing the supplied interface to JSON.
//
// Parameters:
//   - ctx - the context
//   - key - the session key
//   - value - the value to set
//   - seconds - the number of seconds until session expires
//   - options - the session options
//
// Returns:
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) SetAny(ctx context.Context, key string, value any, seconds int64, options SessionOptionsInterface) error {
	jsonValue, jsonError := json.Marshal(value)
	if jsonError != nil {
		return jsonError
	}

	return st.Set(ctx, key, string(jsonValue), seconds, options)
}

// SetMap sets a session value by serializing the supplied map to JSON.
//
// Parameters:
//   - ctx - the context
//   - key - the session key
//   - value - the map to set
//   - seconds - the number of seconds until session expires
//   - options - the session options
//
// Returns:
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) SetMap(ctx context.Context, key string, value map[string]any, seconds int64, options SessionOptionsInterface) error {
	jsonValue, jsonError := json.Marshal(value)
	if jsonError != nil {
		return jsonError
	}

	return st.Set(ctx, key, string(jsonValue), seconds, options)
}

// sessionSelectQuery builds a SQL select query for sessions based on the provided options.
//
// Parameters:
//   - options - the session query options
//
// Returns:
//   - *goqu.SelectDataset - the select dataset
//   - []any - the columns to select
//   - error - nil if successful, otherwise an error
func (st *storeImplementation) sessionSelectQuery(options SessionQueryInterface) (selectDataset *goqu.SelectDataset, columns []any, err error) {
	if options == nil {
		return nil, []any{}, errors.New("session query: cannot be nil")
	}

	if err := options.Validate(); err != nil {
		return nil, []any{}, err
	}

	q := goqu.Dialect(st.dbDriverName).From(st.sessionTableName)

	if options.HasCreatedAtGte() && options.HasCreatedAtLte() {
		q = q.Where(
			goqu.C(COLUMN_CREATED_AT).Gte(options.CreatedAtGte()),
			goqu.C(COLUMN_CREATED_AT).Lte(options.CreatedAtLte()),
		)
	} else if options.HasCreatedAtGte() {
		q = q.Where(goqu.C(COLUMN_CREATED_AT).Gte(options.CreatedAtGte()))
	} else if options.HasCreatedAtLte() {
		q = q.Where(goqu.C(COLUMN_CREATED_AT).Lte(options.CreatedAtLte()))
	}

	if options.HasExpiresAtGte() && options.HasExpiresAtLte() {
		q = q.Where(
			goqu.C(COLUMN_EXPIRES_AT).Gte(options.ExpiresAtGte()),
			goqu.C(COLUMN_EXPIRES_AT).Lte(options.ExpiresAtLte()),
		)
	} else if options.HasExpiresAtGte() {
		q = q.Where(goqu.C(COLUMN_EXPIRES_AT).Gte(options.ExpiresAtGte()))
	} else if options.HasExpiresAtLte() {
		q = q.Where(goqu.C(COLUMN_EXPIRES_AT).Lte(options.ExpiresAtLte()))
	}

	if options.HasID() {
		q = q.Where(goqu.C(COLUMN_ID).Eq(options.ID()))
	}

	if options.HasIDIn() {
		q = q.Where(goqu.C(COLUMN_ID).In(options.IDIn()))
	}

	if options.HasKey() {
		q = q.Where(goqu.C(COLUMN_SESSION_KEY).Eq(options.Key()))
	}

	if options.HasUserAgent() {
		q = q.Where(goqu.C(COLUMN_USER_AGENT).Eq(options.UserAgent()))
	}

	if options.HasUserID() {
		q = q.Where(goqu.C(COLUMN_USER_ID).Eq(options.UserID()))
	}

	if options.HasUserIpAddress() {
		q = q.Where(goqu.C(COLUMN_IP_ADDRESS).Eq(options.UserIpAddress()))
	}

	if !options.IsCountOnly() {
		if options.HasLimit() {
			q = q.Limit(uint(options.Limit()))
		}

		if options.HasOffset() {
			q = q.Offset(uint(options.Offset()))
		}
	}

	sortOrder := sb.DESC
	if options.HasSortOrder() && options.SortOrder() != "" {
		sortOrder = options.SortOrder()
	}

	if options.HasOrderBy() && options.OrderBy() != "" {
		if strings.EqualFold(sortOrder, sb.ASC) {
			q = q.Order(goqu.I(options.OrderBy()).Asc())
		} else {
			q = q.Order(goqu.I(options.OrderBy()).Desc())
		}
	}

	columns = []any{}

	for _, column := range options.Columns() {
		columns = append(columns, column)
	}

	if options.SoftDeletedIncluded() {
		return q, columns, nil // soft deleted sessions requested specifically
	}

	softDeleted := goqu.C(COLUMN_SOFT_DELETED_AT).
		Gt(carbon.Now(carbon.UTC).ToDateTimeString())

	return q.Where(softDeleted), columns, nil
}

// logSql logs SQL statements if debug is enabled.
//
// Parameters:
//   - sqlOperationType - the type of SQL operation
//   - sql - the SQL statement
//   - params - the SQL parameters
func (st *storeImplementation) logSql(sqlOperationType string, sql string, params ...interface{}) {
	if !st.debugEnabled {
		return
	}

	if st.sqlLogger != nil {
		st.sqlLogger.Debug("sql: "+sqlOperationType, slog.String("sql", sql), slog.Any("params", params))
	}
}

func (st *storeImplementation) encryptValue(value string) (string, error) {
	if st == nil || st.encryptor == nil {
		return value, nil
	}

	return st.encryptor.encrypt(value)
}

func (st *storeImplementation) decryptValue(value string) (string, error) {
	if st == nil || st.encryptor == nil {
		return value, nil
	}

	return st.encryptor.decrypt(value)
}
