package entities

import (
	"encoding/json"
	"time"
)

type Document struct {
	ID        string           `json:"id" db:"id"`
	Name      string           `json:"name" db:"name"`
	OwnerID   string           `json:"owner_id" db:"owner_id"`
	MIME      string           `json:"mime" db:"mime"`
	IsFile    bool             `json:"file" db:"is_file"`
	IsPublic  bool             `json:"public" db:"is_public"`
	FilePath  *string          `json:"-" db:"file_path"`
	JSONData  *json.RawMessage `json:"json,omitempty" db:"json_data"`
	Grant     *[]string        `json:"grant" db:"grant"`
	CreatedAt time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt time.Time        `json:"updated_at" db:"updated_at"`
}

type DocumentFilter struct {
	OwnerID string
	Key     string
	Value   string
	Limit   int
}
