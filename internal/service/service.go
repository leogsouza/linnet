package service

import (
	"database/sql"

	"github.com/hako/branca"
)

// Service contains the core logic. You can use it to back REST, GraphQL or RPC API
type Service struct {
	DB    *sql.DB
	Codec *branca.Branca
}
