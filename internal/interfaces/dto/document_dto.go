package dto

import (
	"document-server/internal/domain/entities"
	"encoding/json"
	"mime/multipart"
)

type DocumentMeta struct {
	Name   string   `json:"name" binding:"required"`
	File   bool     `json:"file"`
	Public bool     `json:"public"`
	Token  string   `json:"token" binding:"required"`
	MIME   string   `json:"mime"`
	Grant  []string `json:"grant"`
}

type DocumentCreateRequest struct {
	Meta *DocumentMeta         `json:"meta"`
	JSON json.RawMessage       `json:"json,omitempty"`
	File *multipart.FileHeader `json:"file,omitempty"`
}

type DocumentCreateResponse struct {
	JSON *json.RawMessage `json:"json,omitempty"`
	File string           `json:"file,omitempty"`
}

type DocumentListRequest struct {
	Token string `form:"token" binding:"required"`
	Login string `form:"login,omitempty"`
	Key   string `form:"key,omitempty"`
	Value string `form:"value,omitempty"`
	Limit int    `form:"limit,omitempty"`
}

type DocumentListResponse struct {
	Docs []*entities.Document `json:"docs"`
}

type DocumentDeleteResponse struct {
	ID      string `json:"id"`
	Success bool   `json:"success"`
}
