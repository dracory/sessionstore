package sessionstore

import "github.com/dromara/carbon/v2"

type SessionInterface interface {
	// From data object

	Data() map[string]string
	DataChanged() map[string]string
	MarkAsNotDirty(...string)

	// Methods

	IsExpired() bool
	IsSoftDeleted() bool

	// Setters and Getters

	GetID() string
	SetID(id string) SessionInterface

	GetKey() string
	SetKey(key string) SessionInterface

	GetUserID() string
	SetUserID(userID string) SessionInterface

	GetIPAddress() string
	SetIPAddress(ipAddress string) SessionInterface

	GetUserAgent() string
	SetUserAgent(userAgent string) SessionInterface

	GetValue() string
	SetValue(value string) SessionInterface

	GetExpiresAt() string
	GetExpiresAtCarbon() *carbon.Carbon
	SetExpiresAt(expiresAt string) SessionInterface

	GetCreatedAt() string
	GetCreatedAtCarbon() *carbon.Carbon
	SetCreatedAt(createdAt string) SessionInterface

	GetUpdatedAt() string
	GetUpdatedAtCarbon() *carbon.Carbon
	SetUpdatedAt(updatedAt string) SessionInterface

	GetSoftDeletedAt() string
	GetSoftDeletedAtCarbon() *carbon.Carbon
	SetSoftDeletedAt(deletedAt string) SessionInterface
}
