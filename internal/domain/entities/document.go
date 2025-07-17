package entities

import (
	"encoding/json"
	"time"
)

type Document struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	OwnerID   string           `json:"owner_id"`
	MIME      string           `json:"mime"`
	IsFile    bool             `json:"file"`
	IsPublic  bool             `json:"public"`
	FilePath  *string          `json:"file_path,omitempty"`
	JSONData  *json.RawMessage `json:"json,omitempty"`
	Grant     *[]string        `json:"grant"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

type DocumentFilter struct {
	OwnerID             string
	RequestingUserLogin string
	Key                 string
	Value               string
	Limit               int
}
