package sessionstore

import (
	"testing"

	"github.com/dromara/carbon/v2"
)

func TestNewSession_DefaultValues(t *testing.T) {
	sess := NewSession()

	if sess.GetID() == "" {
		t.Fatalf("expected generated id")
	}

	if len(sess.GetKey()) != 100 {
		t.Fatalf("expected generated key length 100, got %d", len(sess.GetKey()))
	}

	if sess.GetUserID() != "" {
		t.Fatalf("expected empty user id")
	}

	if sess.GetUserAgent() != "" {
		t.Fatalf("expected empty user agent")
	}

	if sess.GetIPAddress() != "" {
		t.Fatalf("expected empty ip address")
	}

	if sess.GetValue() != "" {
		t.Fatalf("expected empty session value")
	}

	expires := sess.GetExpiresAtCarbon()
	if expires == nil || !expires.Gt(carbon.Now(carbon.UTC)) {
		t.Fatalf("expected expires at in the future, got %v", expires)
	}

	if sess.IsExpired() {
		t.Fatalf("expected session to be active")
	}

	if sess.IsSoftDeleted() {
		t.Fatalf("expected session not to be soft deleted")
	}

	// SoftDeletesMaxDate uses zero time as the max-date sentinel
	if !sess.GetSoftDeletedAtCarbon().IsZero() {
		t.Fatalf("expected soft deleted at to be zero (max-date sentinel)")
	}
}

func TestSession_SettersAndCarbonAccessors(t *testing.T) {
	sess := NewSession()

	expiresAt := carbon.Now(carbon.UTC).AddHours(4).ToDateTimeString(carbon.UTC)
	createdAt := carbon.Now(carbon.UTC).SubHours(1).ToDateTimeString(carbon.UTC)
	updatedAt := carbon.Now(carbon.UTC).ToDateTimeString(carbon.UTC)
	softDeletedAt := carbon.Now(carbon.UTC).AddHours(12).ToDateTimeString(carbon.UTC)

	sess.SetValue("payload-json")
	sess.SetUserID("user-123")
	sess.SetUserAgent("Mozilla/5.0")
	sess.SetIPAddress("127.0.0.1")
	sess.SetExpiresAt(expiresAt)
	sess.SetCreatedAt(createdAt)
	sess.SetUpdatedAt(updatedAt)
	sess.SetSoftDeletedAt(softDeletedAt)

	if sess.GetValue() != "payload-json" {
		t.Fatalf("expected value to match setter")
	}

	if sess.GetUserID() != "user-123" {
		t.Fatalf("expected user id to match setter")
	}

	if sess.GetUserAgent() != "Mozilla/5.0" {
		t.Fatalf("expected user agent to match setter")
	}

	if sess.GetIPAddress() != "127.0.0.1" {
		t.Fatalf("expected ip address to match setter")
	}

	if sess.GetExpiresAt() != expiresAt {
		t.Fatalf("expected expires at to match setter")
	}

	if sess.GetExpiresAtCarbon().ToDateTimeString(carbon.UTC) != expiresAt {
		t.Fatalf("expected carbon expires at to match setter")
	}

	if sess.GetCreatedAt() != createdAt {
		t.Fatalf("expected created at to match setter")
	}

	if sess.GetCreatedAtCarbon().ToDateTimeString(carbon.UTC) != createdAt {
		t.Fatalf("expected carbon created at to match setter")
	}

	if sess.GetUpdatedAt() != updatedAt {
		t.Fatalf("expected updated at to match setter")
	}

	if sess.GetUpdatedAtCarbon().ToDateTimeString(carbon.UTC) != updatedAt {
		t.Fatalf("expected carbon updated at to match setter")
	}

	if sess.GetSoftDeletedAt() != softDeletedAt {
		t.Fatalf("expected soft deleted at to match setter")
	}

	if sess.GetSoftDeletedAtCarbon().ToDateTimeString(carbon.UTC) != softDeletedAt {
		t.Fatalf("expected carbon soft deleted at to match setter")
	}
}

func TestNewSessionFromExistingData(t *testing.T) {
	expiresAt := carbon.Now(carbon.UTC).AddHours(1).ToDateTimeString(carbon.UTC)
	softDeletedAt := carbon.Now(carbon.UTC).SubHours(1).ToDateTimeString(carbon.UTC)

	data := map[string]string{
		COLUMN_ID:              "session-id",
		COLUMN_SESSION_KEY:     "session-key",
		COLUMN_SESSION_VALUE:   "value",
		COLUMN_USER_ID:         "user-id",
		COLUMN_USER_AGENT:      "agent",
		COLUMN_IP_ADDRESS:      "10.1.1.1",
		COLUMN_EXPIRES_AT:      expiresAt,
		COLUMN_CREATED_AT:      carbon.Now(carbon.UTC).SubHours(2).ToDateTimeString(carbon.UTC),
		COLUMN_UPDATED_AT:      carbon.Now(carbon.UTC).SubMinutes(5).ToDateTimeString(carbon.UTC),
		COLUMN_SOFT_DELETED_AT: softDeletedAt,
	}

	sess := NewSessionFromExistingData(data)

	if sess.GetID() != "session-id" {
		t.Fatalf("expected id to match hydrated data")
	}

	if sess.GetKey() != "session-key" {
		t.Fatalf("expected key to match hydrated data")
	}

	if sess.GetValue() != "value" {
		t.Fatalf("expected value to match hydrated data")
	}

	if sess.GetUserID() != "user-id" {
		t.Fatalf("expected user id to match hydrated data")
	}

	if sess.GetUserAgent() != "agent" {
		t.Fatalf("expected user agent to match hydrated data")
	}

	if sess.GetIPAddress() != "10.1.1.1" {
		t.Fatalf("expected ip address to match hydrated data")
	}

	if !sess.IsSoftDeleted() {
		t.Fatalf("expected session to be soft deleted based on hydrated data")
	}

	if sess.IsExpired() {
		t.Fatalf("expected session not to be expired yet")
	}

	if got := sess.GetExpiresAt(); got != expiresAt {
		t.Fatalf("expected expires at %s, got %s", expiresAt, got)
	}
}
