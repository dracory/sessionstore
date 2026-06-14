package sessionstore

// Column names for the session table
const (
	COLUMN_CREATED_AT      = "created_at"
	COLUMN_EXPIRES_AT      = "expires_at"
	COLUMN_ID              = "id"
	COLUMN_IP_ADDRESS      = "ip_address"
	COLUMN_SESSION_KEY     = "session_key"
	COLUMN_SESSION_VALUE   = "session_value"
	COLUMN_SOFT_DELETED_AT = "soft_deleted_at"
	COLUMN_UPDATED_AT      = "updated_at"
	COLUMN_USER_AGENT      = "user_agent"
	COLUMN_USER_ID         = "user_id"
)

// MAX_DATETIME is a far-future datetime used as the default soft-delete sentinel.
const MAX_DATETIME = "2999-12-31 23:59:59"
