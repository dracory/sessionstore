package sessionstore

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/dromara/carbon/v2"
	_ "modernc.org/sqlite"
)

func initDB() (*sql.DB, error) {
	dsn := ":memory:?parseTime=true"
	db, err := sql.Open("sqlite", dsn)

	if err != nil {
		return nil, err
	}

	return db, nil
}

func initStore() (StoreInterface, error) {
	store, err := initStoreWithOptions(NewStoreOptions{
		SessionTableName:   "session",
		AutomigrateEnabled: true,
	})

	if err != nil {
		return nil, err
	}

	return store, nil
}

func initStoreWithOptions(opts NewStoreOptions) (StoreInterface, error) {
	db, err := initDB()

	if err != nil {
		return nil, err
	}

	opts.DB = db

	if opts.SessionTableName == "" {
		opts.SessionTableName = "session"
	}

	store, err := NewStore(opts)

	if err != nil {
		db.Close()
		return nil, err
	}

	if store == nil {
		db.Close()
		return nil, errors.New("unexpected nil store")
	}

	return store, nil
}

func TestStore_Create(t *testing.T) {
	store, err := initStore()

	if err != nil {
		t.Fatal("Store could not be created: ", err.Error())
	}

	if store == nil {
		t.Fatal("unexpected nil store")
	}
}

func TestStore_Automigrate(t *testing.T) {
	store, err := initStore()

	if err != nil {
		t.Fatal("Store could not be created: ", err.Error())
	}

	err = store.MigrateUp(context.Background())

	if err != nil {
		t.Fatal("MigrateUp failed: " + err.Error())
	}
}

func TestStore_EnableDebug(t *testing.T) {
	store, err := initStore()

	if err != nil {
		t.Fatal("Store could not be created: ", err.Error())
	}

	store.EnableDebug(true)

	err = store.MigrateUp(context.Background())

	if err != nil {
		t.Fatal("MigrateUp failed: " + err.Error())
	}
}

// func TestSetGetMap(t *testing.T) {
// 	store, err := initStore()

// 	if err != nil {
// 		t.Fatal("Store could not be created: ", err.Error())
// 	}

// 	value := map[string]any{
// 		"key1": "value1",
// 		"key2": "value2",
// 		"key3": "value3",
// 	}
// 	err = store.SetMap("mykey", value, 5, SessionOptions{})

// 	if err != nil {
// 		t.Fatalf("Set Map failed: " + err.Error())
// 	}

// 	result, err := store.GetMap("mykey", nil, SessionOptions{})

// 	if err != nil {
// 		t.Fatalf("Get JSON failed: " + err.Error())
// 	}

// 	if result == nil {
// 		t.Fatalf("GetMap failed: nil returned")
// 	}

// 	if result["key1"].(string) != value["key1"] {
// 		t.Fatalf("Key1 not correct: " + result["key1"].(string))
// 	}

// 	if result["key2"] != value["key2"] {
// 		t.Fatalf("Key2 not correct: " + result["key2"].(string))
// 	}

// 	if result["key3"] != value["key3"] {
// 		t.Fatalf("Key3 not correct: " + result["key3"].(string))
// 	}
// }

// func TestMergeMap(t *testing.T) {
// 	store, err := initStore()

// 	if err != nil {
// 		t.Fatal("Store could not be created: ", err.Error())
// 	}

// 	value := map[string]any{
// 		"key1": "value1",
// 		"key2": "value2",
// 		"key3": "value3",
// 	}
// 	err = store.SetMap("mykey", value, 600, SessionOptions{})

// 	if err != nil {
// 		t.Fatalf("Set Map failed: " + err.Error())
// 	}

// 	valueMerge := map[string]any{
// 		"key2": "value22",
// 		"key3": "value33",
// 	}

// 	err = store.MergeMap("mykey", valueMerge, 600, SessionOptions{})

// 	if err != nil {
// 		t.Fatalf("Merge Map failed: " + err.Error())
// 	}

// 	result, err := store.GetMap("mykey", nil, SessionOptions{})

// 	if err != nil {
// 		t.Fatalf("Get JSON failed: " + err.Error())
// 	}

// 	if result == nil {
// 		t.Fatalf("GetMap failed: nil returned")
// 	}

// 	if result["key1"].(string) != value["key1"] {
// 		t.Fatalf("Key1 not correct: " + result["key1"].(string))
// 	}

// 	if result["key2"].(string) != valueMerge["key2"] {
// 		t.Fatalf("Key2 not correct: " + result["key2"].(string))
// 	}

// 	if result["key3"].(string) != valueMerge["key3"] {
// 		t.Fatalf("Key3 not correct: " + result["key3"].(string))
// 	}
// }

// func TestExtend(t *testing.T) {
// 	store, err := initStore()

// 	if err != nil {
// 		t.Fatal("Store could not be created: ", err.Error())
// 	}

// 	err = store.Set("mykey", "test", 5, SessionOptions{})

// 	if err != nil {
// 		t.Fatal("Set failed: " + err.Error())
// 	}

// 	err = store.Extend("mykey", 100, SessionOptions{})

// 	if err != nil {
// 		t.Fatal("Extend failed: " + err.Error())
// 	}

// 	sessionExtended, err := store.FindByKey("mykey", SessionOptions{})

// 	if err != nil {
// 		t.Fatal("Extend failed: " + err.Error())
// 	}

// 	if sessionExtended == nil {
// 		t.Fatal("Extend failed. Session is NIL")
// 	}

// 	if sessionExtended.GetValue() != "test" {
// 		t.Fatal("Extend failed. Value is wrong", sessionExtended.GetValue())
// 	}

// 	diff := sessionExtended.GetExpiresAtCarbon().DiffAbsInSeconds(carbon.Now(carbon.UTC))

// 	if diff < 90 {
// 		t.Fatal("Extend failed. ExpiresAt must be more than 90 seconds", sessionExtended.GetExpiresAt(), diff)
// 	}

// 	if diff > 110 {
// 		t.Fatal("Extend failed. ExpiresAt must be less than 110 seconds", sessionExtended.GetExpiresAt(), diff)
// 	}

// }

func TestStore_SessionCreate(t *testing.T) {
	store, err := initStore()

	if err != nil {
		t.Fatal("Store could not be created: ", err.Error())
	}

	session := NewSession()

	if session == nil {
		t.Fatal("unexpected nil session")
	}

	if session.GetID() == "" {
		t.Fatal("unexpected empty id:", session.GetID())
	}

	if len(session.GetID()) < 8 {
		t.Fatal("unexpected id length:", len(session.GetID()))
	}

	if session.GetKey() == "" {
		t.Fatal("unexpected empty key:", session.GetKey())
	}

	if len(session.GetKey()) != 100 {
		t.Fatal("unexpected key length:", len(session.GetKey()))
	}

	err = store.SessionCreate(context.Background(), session)

	if err != nil {
		t.Fatal("unexpected error:", err)
	}
}

func TestStore_SessionDelete(t *testing.T) {
	store, err := initStore()

	if err != nil {
		t.Fatal("Store could not be created: ", err.Error())
	}

	session := NewSession()

	err = store.SessionCreate(context.Background(), session)

	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	err = store.SessionDeleteByID(context.Background(), session.GetID())

	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	sessionFindWithDeleted, err := store.SessionList(context.Background(), SessionQuery().
		SetID(session.GetID()).
		SetLimit(1).
		SetSoftDeletedIncluded(true))

	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if len(sessionFindWithDeleted) != 0 {
		t.Fatal("Session MUST be deleted, but it is not")
	}
}

func TestStore_SessionDeleteByID(t *testing.T) {
	store, err := initStore()

	if err != nil {
		t.Fatal("Store could not be created: ", err.Error())
	}

	session := NewSession().
		SetValue("one two three four")

	if session == nil {
		t.Fatal("unexpected nil session")
	}

	if session.GetID() == "" {
		t.Fatal("unexpected empty id:", session.GetID())
	}

	err = store.SessionCreate(context.Background(), session)

	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	err = store.SessionDeleteByID(context.Background(), session.GetID())

	if err != nil {
		t.Error("unexpected error:", err)
	}

	sessionFindWithDeleted, err := store.SessionList(context.Background(), SessionQuery().
		SetID(session.GetID()).
		SetLimit(1).
		SetSoftDeletedIncluded(true))

	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if len(sessionFindWithDeleted) != 0 {
		t.Fatal("Session MUST be deleted, but it is not")
	}
}

func TestStore_SessionExtend(t *testing.T) {
	store, err := initStore()

	if err != nil {
		t.Fatal("Store could not be created: ", err.Error())
	}

	session := NewSession().
		SetValue("one two three four")

	if session == nil {
		t.Fatal("unexpected nil session")
	}

	if session.GetID() == "" {
		t.Fatal("unexpected empty id:", session.GetID())
	}

	err = store.SessionCreate(context.Background(), session)

	if err != nil {
		t.Error("unexpected error:", err)
	}

	if session.GetExpiresAt() == "" {
		t.Fatal("unexpected empty expiresAt:", session.GetExpiresAt())
	}

	originalExpiresAt := session.GetExpiresAt()
	newExpiresAt := session.GetExpiresAtCarbon().AddSeconds(100)

	if session.GetExpiresAtCarbon().Gte(newExpiresAt) {
		t.Fatal("session expiresAt must be less than new expiresAt:", newExpiresAt, " but is: ", session.GetExpiresAtCarbon())
	}

	err = store.SessionExtend(context.Background(), session, 100)

	if err != nil {
		t.Error("unexpected error:", err)
	}

	sessionExtended, errFind := store.SessionFindByID(context.Background(), session.GetID())

	if errFind != nil {
		t.Fatal("unexpected error:", errFind)
	}

	if sessionExtended == nil {
		t.Fatal("Session MUST NOT be nil")
	}

	if sessionExtended.GetID() != session.GetID() {
		t.Fatal("IDs do not match")
	}

	if sessionExtended.GetValue() != session.GetValue() {
		t.Fatal("Values do not match")
	}

	if sessionExtended.GetValue() != "one two three four" {
		t.Fatal("Values do not match")
	}

	if sessionExtended.GetExpiresAt() == "" {
		t.Fatal("unexpected empty expiresAt:", session.GetExpiresAt())
	}

	if sessionExtended.GetExpiresAt() == originalExpiresAt {
		t.Fatal("unexpected same expiresAt:", originalExpiresAt)
	}

	if sessionExtended.GetExpiresAt() == sessionExtended.GetCreatedAt() {
		t.Fatal("unexpected same expiresAt:", session.GetExpiresAt())
	}

	if sessionExtended.GetExpiresAt() == sessionExtended.GetUpdatedAt() {
		t.Fatal("unexpected same expiresAt:", session.GetExpiresAt())
	}

	if sessionExtended.GetExpiresAtCarbon().Gte(newExpiresAt) {
		t.Fatal("expiresAt must be more than or equal to:", newExpiresAt, " but is: ", sessionExtended.GetExpiresAtCarbon())
	}

	diff := sessionExtended.GetExpiresAtCarbon().DiffAbsInSeconds(carbon.Now(carbon.UTC))

	if diff < 90 {
		t.Fatal("Extend failed. ExpiresAt must be more than 90 seconds", sessionExtended.GetExpiresAt(), diff)
	}

	if diff > 110 {
		t.Fatal("Extend failed. ExpiresAt must be less than 110 seconds", sessionExtended.GetExpiresAt(), diff)
	}
}

func TestStore_SessionFindByID(t *testing.T) {
	store, err := initStore()

	if err != nil {
		t.Fatal("Store could not be created: ", err.Error())
	}

	session := NewSession().
		SetValue("one two three four")

	if session == nil {
		t.Fatal("unexpected nil session")
	}

	if session.GetID() == "" {
		t.Fatal("unexpected empty id:", session.GetID())
	}

	err = store.SessionCreate(context.Background(), session)
	if err != nil {
		t.Error("unexpected error:", err)
	}

	sessionFound, errFind := store.SessionFindByID(context.Background(), session.GetID())

	if errFind != nil {
		t.Fatal("unexpected error:", errFind)
	}

	if sessionFound == nil {
		t.Fatal("Session MUST NOT be nil")
	}

	if sessionFound.GetID() != session.GetID() {
		t.Fatal("IDs do not match")
	}

	if sessionFound.GetValue() != session.GetValue() {
		t.Fatal("Values do not match")
	}

	if sessionFound.GetValue() != "one two three four" {
		t.Fatal("Values do not match")
	}
}

func TestStore_SessionFindByKey(t *testing.T) {
	store, err := initStore()

	if err != nil {
		t.Fatal("Store could not be created: ", err.Error())
	}

	session := NewSession().
		SetValue("one two three four")

	if session == nil {
		t.Fatal("unexpected nil session")
	}

	if session.GetKey() == "" {
		t.Fatal("unexpected empty key:", session.GetKey())
	}

	err = store.SessionCreate(context.Background(), session)
	if err != nil {
		t.Error("unexpected error:", err)
	}

	sessionFound, errFind := store.SessionFindByKey(context.Background(), session.GetKey())

	if errFind != nil {
		t.Fatal("unexpected error:", errFind)
	}

	if sessionFound == nil {
		t.Fatal("Session MUST NOT be nil")
	}

	if sessionFound.GetID() != session.GetID() {
		t.Fatal("IDs do not match")
	}

	if sessionFound.GetValue() != session.GetValue() {
		t.Fatal("Values do not match")
	}

	if sessionFound.GetValue() != "one two three four" {
		t.Fatal("Values do not match")
	}
}

func TestStore_SessionList(t *testing.T) {
	store, err := initStore()

	if err != nil {
		t.Fatal("Store could not be created: ", err.Error())
	}

	session1 := NewSession().
		SetUserID("1").
		SetValue("one two three")

	session2 := NewSession().
		SetUserID("2").
		SetValue("four five six")

	session3 := NewSession().
		SetUserID("3").
		SetValue("seven eight nine")

	for _, session := range []SessionInterface{session1, session2, session3} {
		err = store.SessionCreate(context.Background(), session)
		if err != nil {
			t.Error("unexpected error:", err)
		}
	}

	sessionList, errList := store.SessionList(context.Background(), SessionQuery().
		SetUserID("2").
		SetLimit(2))

	if errList != nil {
		t.Fatal("unexpected error:", errList)
	}

	if len(sessionList) != 1 {
		t.Fatal("unexpected session list length:", len(sessionList))
	}
}

func TestStore_SessionSoftDelete(t *testing.T) {
	store, err := initStore()

	if err != nil {
		t.Fatal("Store could not be created: ", err.Error())
	}

	session := NewSession()

	err = store.SessionCreate(context.Background(), session)

	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	err = store.SessionSoftDeleteByID(context.Background(), session.GetID())

	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	// SoftDeletesMaxDate uses zero time as the max-date sentinel
	if !session.GetSoftDeletedAtCarbon().IsZero() {
		t.Fatal("Session MUST NOT be soft deleted")
	}

	sessionFound, errFind := store.SessionFindByID(context.Background(), session.GetID())

	if errFind != nil {
		t.Fatal("unexpected error:", errFind)
	}

	if sessionFound != nil {
		t.Fatal("Session MUST be nil")
	}

	sessionFindWithSoftDeleted, err := store.SessionList(context.Background(), SessionQuery().
		SetID(session.GetID()).
		SetSoftDeletedIncluded(true).
		SetLimit(1))

	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if len(sessionFindWithSoftDeleted) == 0 {
		t.Fatal("Exam MUST be soft deleted")
	}

	// SoftDeletesMaxDate uses zero time as the max-date sentinel for active records
	// Soft deleted records will have a recent timestamp
	if sessionFindWithSoftDeleted[0].GetSoftDeletedAtCarbon().IsZero() {
		t.Fatal("Session MUST be soft deleted (should have timestamp, not zero)")
	}

	if !sessionFindWithSoftDeleted[0].IsSoftDeleted() {
		t.Fatal("Session MUST be soft deleted")
	}
}

func TestStore_SessionUpdate(t *testing.T) {
	store, err := initStore()

	if err != nil {
		t.Fatal("Store could not be created: ", err.Error())
	}

	session := NewSession()

	err = store.SessionCreate(context.Background(), session)

	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	session.SetValue("one two three")

	err = store.SessionUpdate(context.Background(), session)

	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	sessionFound, errFind := store.SessionFindByID(context.Background(), session.GetID())

	if errFind != nil {
		t.Fatal("unexpected error:", errFind)
	}

	if sessionFound == nil {
		t.Fatal("Session MUST NOT be nil")
	}

	if sessionFound.GetValue() != "one two three" {
		t.Fatal("Value MUST be 'one two three', found: ", sessionFound.GetValue())
	}
}

func TestStore_SetGetWithoutEncryption(t *testing.T) {
	store, err := initStoreWithOptions(NewStoreOptions{
		SessionTableName:   "session",
		AutomigrateEnabled: true,
	})

	if err != nil {
		t.Fatal("Store could not be created: ", err.Error())
	}

	defer store.GetDB().Close()

	sessionKey := "plain-key"
	value := "plain-value"

	session := NewSession().
		SetKey(sessionKey).
		SetValue(value)

	if err := store.SessionCreate(context.Background(), session); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	got, err := store.SessionFindByKey(context.Background(), sessionKey)

	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.GetValue() != value {
		t.Fatalf("expected value %q, got %q", value, got)
	}

	var raw string
	rowErr := store.GetDB().QueryRow("SELECT session_value FROM session WHERE session_key = ?", sessionKey).Scan(&raw)

	if rowErr != nil {
		t.Fatalf("failed to query raw value: %v", rowErr)
	}

	if raw != value {
		t.Fatalf("expected raw stored value %q, got %q", value, raw)
	}
}

func TestStore_SetGetWithEncryption(t *testing.T) {
	encryptionKey := []byte("0123456789abcdef0123456789abcdef")

	store, err := initStoreWithOptions(NewStoreOptions{
		SessionTableName:   "session",
		AutomigrateEnabled: true,
		EncryptionEnabled:  true,
		EncryptionKey:      encryptionKey,
	})

	if err != nil {
		t.Fatal("Store could not be created: ", err.Error())
	}

	defer store.GetDB().Close()

	sessionKey := "encrypted-key"
	value := "emoji 😀 and json {\"foo\":\"bar\"}"

	session := NewSession().
		SetKey(sessionKey).
		SetValue(value)

	if err := store.SessionCreate(context.Background(), session); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	got, err := store.SessionFindByKey(context.Background(), sessionKey)

	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.GetValue() != value {
		t.Fatalf("expected value %q, got %q", value, got)
	}

	sessionFound, err := store.SessionFindByKey(context.Background(), sessionKey)
	if err != nil {
		t.Fatalf("FindByKey failed: %v", err)
	}

	if sessionFound == nil {
		t.Fatal("expected session to be found")
	}

	if sessionFound.GetValue() != value {
		t.Fatalf("expected session value %q, got %q", value, sessionFound.GetValue())
	}

	var raw string
	rowErr := store.GetDB().QueryRow("SELECT session_value FROM session WHERE session_key = ?", sessionKey).Scan(&raw)

	if rowErr != nil {
		t.Fatalf("failed to query raw value: %v", rowErr)
	}

	if raw == value {
		t.Fatalf("expected stored value to be encrypted, but matched plaintext")
	}

	if !strings.HasPrefix(raw, encryptedValuePrefix) {
		t.Fatalf("expected stored value to have encryption prefix, got %q", raw)
	}
}

func TestNewStore_EncryptionEnabledWithoutKey(t *testing.T) {
	store, err := initStoreWithOptions(NewStoreOptions{
		SessionTableName:   "session",
		AutomigrateEnabled: true,
		EncryptionEnabled:  true,
	})

	if err == nil {
		if store != nil {
			store.GetDB().Close()
		}
		t.Fatal("expected error when encryption enabled without key")
	}

	if !strings.Contains(err.Error(), "encryption key is required") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
