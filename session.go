package sessionstore

import (
	"database/sql"
	"time"

	"github.com/dracory/neat/database/orm"
	neatuid "github.com/dracory/neat/support/uid"
	"github.com/dromara/carbon/v2"
)

var _ SessionInterface = (*sessionImplementation)(nil)

// == TYPE ===================================================================

// sessionImplementation is the private implementation of SessionInterface.
type sessionImplementation struct {
	orm.ShortID

	KeyField         string    `db:"session_key"`
	UserIDField      string    `db:"user_id"`
	IPAddressField   string    `db:"ip_address"`
	UserAgentField   string    `db:"user_agent"`
	ValueField       string    `db:"session_value"`
	ExpiresAtField   time.Time `db:"expires_at"`
	CreatedAtField   orm.CreatedAt
	UpdatedAtField   orm.UpdatedAt
	SoftDeletedField sql.NullTime `db:"soft_deleted_at"`
}

// == CONSTRUCTORS ============================================================

// NewSession creates a new session.
func NewSession() SessionInterface {
	o := &sessionImplementation{}
	o.SetID(neatuid.GenerateShortID())
	o.SetKey(generateSessionKey(100))
	o.SetValue("")
	o.SetUserID("")
	o.SetUserAgent("")
	o.SetIPAddress("")
	o.SetExpiresAt(carbon.Now(carbon.UTC).AddHours(2).ToDateTimeString(carbon.UTC))
	o.SetCreatedAt(carbon.Now(carbon.UTC).ToDateTimeString(carbon.UTC))
	o.SetUpdatedAt(carbon.Now(carbon.UTC).ToDateTimeString(carbon.UTC))
	o.SetSoftDeletedAt(MAX_DATETIME)
	return o
}

// NewSessionFromExistingData creates a new session from a raw column map (e.g. query results).
func NewSessionFromExistingData(data map[string]string) SessionInterface {
	o := &sessionImplementation{}
	o.SetID(data[COLUMN_ID])
	o.SetKey(data[COLUMN_SESSION_KEY])
	o.SetUserID(data[COLUMN_USER_ID])
	o.SetIPAddress(data[COLUMN_IP_ADDRESS])
	o.SetUserAgent(data[COLUMN_USER_AGENT])
	o.SetValue(data[COLUMN_SESSION_VALUE])
	if v, ok := data[COLUMN_EXPIRES_AT]; ok {
		o.SetExpiresAt(v)
	}
	if v, ok := data[COLUMN_CREATED_AT]; ok {
		o.SetCreatedAt(v)
	}
	if v, ok := data[COLUMN_UPDATED_AT]; ok {
		o.SetUpdatedAt(v)
	}
	if v, ok := data[COLUMN_SOFT_DELETED_AT]; ok {
		o.SetSoftDeletedAt(v)
	}
	return o
}

// == METHODS =================================================================

// IsExpired returns true if the session is expired.
func (o *sessionImplementation) IsExpired() bool {
	return o.ExpiresAtField.Before(time.Now().UTC())
}

// IsSoftDeleted returns true if the session is soft deleted.
func (o *sessionImplementation) IsSoftDeleted() bool {
	if !o.SoftDeletedField.Valid {
		return false
	}
	return o.SoftDeletedField.Time.Before(time.Now().UTC())
}

// == SETTERS AND GETTERS =====================================================

// GetID returns the id of the session.
func (o *sessionImplementation) GetID() string {
	return o.ShortID.ID
}

// SetID sets the id of the session.
func (o *sessionImplementation) SetID(id string) SessionInterface {
	o.ShortID.ID = id
	return o
}

// GetKey returns the key of the session.
func (o *sessionImplementation) GetKey() string {
	return o.KeyField
}

// SetKey sets the key of the session.
func (o *sessionImplementation) SetKey(key string) SessionInterface {
	o.KeyField = key
	return o
}

// GetUserID returns the user id of the session.
func (o *sessionImplementation) GetUserID() string {
	return o.UserIDField
}

// SetUserID sets the user id of the session.
func (o *sessionImplementation) SetUserID(userID string) SessionInterface {
	o.UserIDField = userID
	return o
}

// GetIPAddress returns the IP address of the session.
func (o *sessionImplementation) GetIPAddress() string {
	return o.IPAddressField
}

// SetIPAddress sets the IP address of the session.
func (o *sessionImplementation) SetIPAddress(iPAddress string) SessionInterface {
	o.IPAddressField = iPAddress
	return o
}

// GetUserAgent returns the user agent of the session.
func (o *sessionImplementation) GetUserAgent() string {
	return o.UserAgentField
}

// SetUserAgent sets the user agent of the session.
func (o *sessionImplementation) SetUserAgent(userAgent string) SessionInterface {
	o.UserAgentField = userAgent
	return o
}

// GetValue returns the value of the session.
func (o *sessionImplementation) GetValue() string {
	return o.ValueField
}

// SetValue sets the value of the session.
func (o *sessionImplementation) SetValue(value string) SessionInterface {
	o.ValueField = value
	return o
}

// GetExpiresAt returns the expires at time of the session as a string.
func (o *sessionImplementation) GetExpiresAt() string {
	if o.ExpiresAtField.IsZero() {
		return ""
	}
	return carbon.CreateFromStdTime(o.ExpiresAtField).ToDateTimeString()
}

// GetExpiresAtCarbon returns the expires at time of the session as a carbon object.
func (o *sessionImplementation) GetExpiresAtCarbon() *carbon.Carbon {
	return carbon.CreateFromStdTime(o.ExpiresAtField)
}

// SetExpiresAt sets the expires at time of the session.
func (o *sessionImplementation) SetExpiresAt(expiresAt string) SessionInterface {
	if expiresAt == "" {
		return o
	}
	o.ExpiresAtField = carbon.Parse(expiresAt, carbon.UTC).StdTime()
	return o
}

// GetCreatedAt returns the created at time of the session.
func (o *sessionImplementation) GetCreatedAt() string {
	if o.CreatedAtField.CreatedAt.IsZero() {
		return ""
	}
	return carbon.CreateFromStdTime(o.CreatedAtField.CreatedAt).ToDateTimeString()
}

// GetCreatedAtCarbon returns the created at time of the session as a carbon object.
func (o *sessionImplementation) GetCreatedAtCarbon() *carbon.Carbon {
	return carbon.CreateFromStdTime(o.CreatedAtField.CreatedAt)
}

// SetCreatedAt sets the created at time of the session.
func (o *sessionImplementation) SetCreatedAt(createdAt string) SessionInterface {
	if createdAt == "" {
		return o
	}
	o.CreatedAtField.CreatedAt = carbon.Parse(createdAt, carbon.UTC).StdTime()
	return o
}

// GetUpdatedAt returns the updated at time of the session.
func (o *sessionImplementation) GetUpdatedAt() string {
	if o.UpdatedAtField.UpdatedAt.IsZero() {
		return ""
	}
	return carbon.CreateFromStdTime(o.UpdatedAtField.UpdatedAt).ToDateTimeString()
}

// GetUpdatedAtCarbon returns the updated at time of the session as a carbon object.
func (o *sessionImplementation) GetUpdatedAtCarbon() *carbon.Carbon {
	return carbon.CreateFromStdTime(o.UpdatedAtField.UpdatedAt)
}

// SetUpdatedAt sets the updated at time of the session.
func (o *sessionImplementation) SetUpdatedAt(updatedAt string) SessionInterface {
	if updatedAt == "" {
		return o
	}
	o.UpdatedAtField.UpdatedAt = carbon.Parse(updatedAt, carbon.UTC).StdTime()
	return o
}

// GetSoftDeletedAt returns the soft deleted at time of the session as a string.
func (o *sessionImplementation) GetSoftDeletedAt() string {
	if !o.SoftDeletedField.Valid || o.SoftDeletedField.Time.IsZero() {
		return ""
	}
	return carbon.CreateFromStdTime(o.SoftDeletedField.Time).ToDateTimeString()
}

// GetSoftDeletedAtCarbon returns the soft deleted at time of the session as a carbon object.
func (o *sessionImplementation) GetSoftDeletedAtCarbon() *carbon.Carbon {
	if !o.SoftDeletedField.Valid {
		return carbon.CreateFromStdTime(time.Time{})
	}
	return carbon.CreateFromStdTime(o.SoftDeletedField.Time)
}

// SetSoftDeletedAt sets the soft deleted at time of the session.
func (o *sessionImplementation) SetSoftDeletedAt(deletedAt string) SessionInterface {
	if deletedAt == "" {
		o.SoftDeletedField = sql.NullTime{Valid: false}
		return o
	}
	t := carbon.Parse(deletedAt, carbon.UTC).StdTime()
	o.SoftDeletedField = sql.NullTime{Time: t, Valid: true}
	return o
}
