package sessionstore

import (
	"github.com/dracory/sb"
)

// SQLCreateTable returns a SQL string for creating the cache table
func (st *storeImplementation) SQLCreateTable() (string, error) {
	sql, err := sb.NewBuilder(st.dbDriverName).
		Table(st.sessionTableName).
		Column(sb.Column{
			Name:       COLUMN_ID,
			Type:       sb.COLUMN_TYPE_STRING,
			Length:     40,
			PrimaryKey: true,
		}).
		Column(sb.Column{
			Name:   COLUMN_SESSION_KEY,
			Type:   sb.COLUMN_TYPE_STRING,
			Length: 255,
		}).
		Column(sb.Column{
			Name:   COLUMN_USER_ID,
			Type:   sb.COLUMN_TYPE_STRING,
			Length: 40,
		}).
		Column(sb.Column{
			Name:   COLUMN_IP_ADDRESS,
			Type:   sb.COLUMN_TYPE_STRING,
			Length: 50,
		}).
		Column(sb.Column{
			Name:   COLUMN_USER_AGENT,
			Type:   sb.COLUMN_TYPE_STRING,
			Length: 1024,
		}).
		Column(sb.Column{
			Name: COLUMN_SESSION_VALUE,
			Type: sb.COLUMN_TYPE_TEXT,
		}).
		Column(sb.Column{
			Name: COLUMN_EXPIRES_AT,
			Type: sb.COLUMN_TYPE_DATETIME,
		}).
		Column(sb.Column{
			Name: COLUMN_CREATED_AT,
			Type: sb.COLUMN_TYPE_DATETIME,
		}).
		Column(sb.Column{
			Name: COLUMN_UPDATED_AT,
			Type: sb.COLUMN_TYPE_DATETIME,
		}).
		Column(sb.Column{
			Name: COLUMN_SOFT_DELETED_AT,
			Type: sb.COLUMN_TYPE_DATETIME,
		}).
		CreateIfNotExists()

	return sql, err
}
