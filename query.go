package sessionstore

import "errors"

// SessionQueryInterface defines the interface for session query operations.
type SessionQueryInterface interface {
	Validate() error

	HasCreatedAtGte() bool
	CreatedAtGte() string
	SetCreatedAtGte(createdAtGte string) SessionQueryInterface

	HasCreatedAtLte() bool
	CreatedAtLte() string
	SetCreatedAtLte(createdAtLte string) SessionQueryInterface

	HasExpiresAtGte() bool
	ExpiresAtGte() string
	SetExpiresAtGte(expiresAtGte string) SessionQueryInterface

	HasExpiresAtLte() bool
	ExpiresAtLte() string
	SetExpiresAtLte(expiresAtLte string) SessionQueryInterface

	HasID() bool
	ID() string
	SetID(id string) SessionQueryInterface

	HasIDIn() bool
	IDIn() []string
	SetIDIn(idIn []string) SessionQueryInterface

	HasKey() bool
	Key() string
	SetKey(key string) SessionQueryInterface

	HasUserID() bool
	UserID() string
	SetUserID(userID string) SessionQueryInterface

	HasUserIpAddress() bool
	UserIpAddress() string
	SetUserIpAddress(userIpAddress string) SessionQueryInterface

	HasUserAgent() bool
	UserAgent() string
	SetUserAgent(userAgent string) SessionQueryInterface

	HasOffset() bool
	Offset() int
	SetOffset(offset int) SessionQueryInterface

	HasLimit() bool
	Limit() int
	SetLimit(limit int) SessionQueryInterface

	HasSortOrder() bool
	SortOrder() string
	SetSortOrder(sortOrder string) SessionQueryInterface

	HasOrderBy() bool
	OrderBy() string
	SetOrderBy(orderBy string) SessionQueryInterface

	HasSoftDeletedIncluded() bool
	SoftDeletedIncluded() bool
	SetSoftDeletedIncluded(withSoftDeleted bool) SessionQueryInterface
}

// SessionQuery is a shortcut for NewSessionQuery.
func SessionQuery() SessionQueryInterface {
	return NewSessionQuery()
}

// NewSessionQuery creates a new session query.
func NewSessionQuery() SessionQueryInterface {
	return &sessionQuery{
		properties: make(map[string]interface{}),
	}
}

var _ SessionQueryInterface = (*sessionQuery)(nil)

type sessionQuery struct {
	properties map[string]interface{}
}

func (q *sessionQuery) Validate() error {
	if q.HasCreatedAtGte() && q.CreatedAtGte() == "" {
		return errors.New("session query: created_at_gte cannot be empty")
	}
	if q.HasCreatedAtLte() && q.CreatedAtLte() == "" {
		return errors.New("session query: created_at_lte cannot be empty")
	}
	if q.HasID() && q.ID() == "" {
		return errors.New("session query: id cannot be empty")
	}
	if q.HasIDIn() && len(q.IDIn()) < 1 {
		return errors.New("session query: id_in cannot be empty array")
	}
	if q.HasLimit() && q.Limit() < 0 {
		return errors.New("session query: limit cannot be negative")
	}
	if q.HasOffset() && q.Offset() < 0 {
		return errors.New("session query: offset cannot be negative")
	}
	return nil
}

func (q *sessionQuery) hasProperty(key string) bool {
	_, ok := q.properties[key]
	return ok
}

func (q *sessionQuery) HasCreatedAtGte() bool { return q.hasProperty("created_at_gte") }
func (q *sessionQuery) CreatedAtGte() string  { return q.properties["created_at_gte"].(string) }
func (q *sessionQuery) SetCreatedAtGte(v string) SessionQueryInterface {
	q.properties["created_at_gte"] = v
	return q
}

func (q *sessionQuery) HasCreatedAtLte() bool { return q.hasProperty("created_at_lte") }
func (q *sessionQuery) CreatedAtLte() string  { return q.properties["created_at_lte"].(string) }
func (q *sessionQuery) SetCreatedAtLte(v string) SessionQueryInterface {
	q.properties["created_at_lte"] = v
	return q
}

func (q *sessionQuery) HasExpiresAtGte() bool { return q.hasProperty("expires_at_gte") }
func (q *sessionQuery) ExpiresAtGte() string  { return q.properties["expires_at_gte"].(string) }
func (q *sessionQuery) SetExpiresAtGte(v string) SessionQueryInterface {
	q.properties["expires_at_gte"] = v
	return q
}

func (q *sessionQuery) HasExpiresAtLte() bool { return q.hasProperty("expires_at_lte") }
func (q *sessionQuery) ExpiresAtLte() string  { return q.properties["expires_at_lte"].(string) }
func (q *sessionQuery) SetExpiresAtLte(v string) SessionQueryInterface {
	q.properties["expires_at_lte"] = v
	return q
}

func (q *sessionQuery) HasID() bool { return q.hasProperty("id") }
func (q *sessionQuery) ID() string  { return q.properties["id"].(string) }
func (q *sessionQuery) SetID(id string) SessionQueryInterface {
	q.properties["id"] = id
	return q
}

func (q *sessionQuery) HasIDIn() bool  { return q.hasProperty("id_in") }
func (q *sessionQuery) IDIn() []string { return q.properties["id_in"].([]string) }
func (q *sessionQuery) SetIDIn(v []string) SessionQueryInterface {
	q.properties["id_in"] = v
	return q
}

func (q *sessionQuery) HasKey() bool { return q.hasProperty("key") }
func (q *sessionQuery) Key() string  { return q.properties["key"].(string) }
func (q *sessionQuery) SetKey(key string) SessionQueryInterface {
	q.properties["key"] = key
	return q
}

func (q *sessionQuery) HasUserID() bool { return q.hasProperty("user_id") }
func (q *sessionQuery) UserID() string  { return q.properties["user_id"].(string) }
func (q *sessionQuery) SetUserID(v string) SessionQueryInterface {
	q.properties["user_id"] = v
	return q
}

func (q *sessionQuery) HasUserIpAddress() bool { return q.hasProperty("user_ip_address") }
func (q *sessionQuery) UserIpAddress() string  { return q.properties["user_ip_address"].(string) }
func (q *sessionQuery) SetUserIpAddress(v string) SessionQueryInterface {
	q.properties["user_ip_address"] = v
	return q
}

func (q *sessionQuery) HasUserAgent() bool { return q.hasProperty("user_agent") }
func (q *sessionQuery) UserAgent() string  { return q.properties["user_agent"].(string) }
func (q *sessionQuery) SetUserAgent(v string) SessionQueryInterface {
	q.properties["user_agent"] = v
	return q
}

func (q *sessionQuery) HasOffset() bool { return q.hasProperty("offset") }
func (q *sessionQuery) Offset() int     { return q.properties["offset"].(int) }
func (q *sessionQuery) SetOffset(v int) SessionQueryInterface {
	q.properties["offset"] = v
	return q
}

func (q *sessionQuery) HasLimit() bool { return q.hasProperty("limit") }
func (q *sessionQuery) Limit() int     { return q.properties["limit"].(int) }
func (q *sessionQuery) SetLimit(v int) SessionQueryInterface {
	q.properties["limit"] = v
	return q
}

func (q *sessionQuery) HasSortOrder() bool { return q.hasProperty("sort_order") }
func (q *sessionQuery) SortOrder() string  { return q.properties["sort_order"].(string) }
func (q *sessionQuery) SetSortOrder(v string) SessionQueryInterface {
	q.properties["sort_order"] = v
	return q
}

func (q *sessionQuery) HasOrderBy() bool { return q.hasProperty("order_by") }
func (q *sessionQuery) OrderBy() string  { return q.properties["order_by"].(string) }
func (q *sessionQuery) SetOrderBy(v string) SessionQueryInterface {
	q.properties["order_by"] = v
	return q
}

func (q *sessionQuery) HasSoftDeletedIncluded() bool { return q.hasProperty("soft_deleted_included") }
func (q *sessionQuery) SoftDeletedIncluded() bool {
	if !q.HasSoftDeletedIncluded() {
		return false
	}
	return q.properties["soft_deleted_included"].(bool)
}
func (q *sessionQuery) SetSoftDeletedIncluded(v bool) SessionQueryInterface {
	q.properties["soft_deleted_included"] = v
	return q
}
