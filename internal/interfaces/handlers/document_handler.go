package handlers

import (
	"document-server/internal/domain/entities"
	"document-server/internal/domain/services"
	"document-server/internal/interfaces/dto"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type DocumentHandler struct {
	documentSvc *services.DocumentService
	authSvc     *services.AuthService
	storagePath string
}

func NewDocumentHandler(
	documentSvc *services.DocumentService,
	authSvc *services.AuthService,
	storagePath string,
) *DocumentHandler {
	return &DocumentHandler{
		documentSvc: documentSvc,
		authSvc:     authSvc,
		storagePath: storagePath,
	}
}

func (h *DocumentHandler) Create(c *gin.Context) {
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil { // 32MB
		respondWithError(c, http.StatusBadRequest, 400, "failed to parse multipart form")
		return
	}

	metaStr := c.Request.FormValue("meta")
	if metaStr == "" {
		respondWithError(c, http.StatusBadRequest, 400, "meta field is required")
		return
	}

	var meta dto.DocumentMeta
	if err := json.Unmarshal([]byte(metaStr), &meta); err != nil {
		respondWithError(c, http.StatusBadRequest, 400, "invalid meta format")
		return
	}

	user, err := h.authSvc.ValidateToken(c.Request.Context(), meta.Token)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	var filePath *string
	var jsonData json.RawMessage

	if meta.File {
		file, fileHeader, err := c.Request.FormFile("file")
		if err != nil {
			respondWithError(c, http.StatusBadRequest, 400, "file is required when file=true")
			return
		}
		defer file.Close()

		if err := os.MkdirAll(h.storagePath, 0755); err != nil {
			respondWithError(c, http.StatusInternalServerError, 500, "failed to create storage directory")
			return
		}

		fileName := uuid.NewString() + filepath.Ext(fileHeader.Filename)
		fullPath := filepath.Join(h.storagePath, fileName)

		dst, err := os.Create(fullPath)
		if err != nil {
			respondWithError(c, http.StatusInternalServerError, 500, "failed to create file")
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			respondWithError(c, http.StatusInternalServerError, 500, "failed to save file")
			return
		}

		filePath = &fullPath
	}

	if jsonStr := c.Request.FormValue("json"); jsonStr != "" {
		jsonData = json.RawMessage(jsonStr)
	}

	doc, err := h.documentSvc.Create(c.Request.Context(),
		user.ID,
		meta.Name,
		meta.MIME,
		meta.File,
		meta.Public,
		filePath,
		&jsonData,
		meta.Grant,
	)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response := dto.DocumentCreateResponse{}
	if doc.JSONData != nil {
		response.JSON = doc.JSONData
	}
	if doc.IsFile {
		response.File = doc.Name
	}

	respondWithSuccess(c, nil, response)
}

func (h *DocumentHandler) GetList(c *gin.Context) {
	var req dto.DocumentListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, 400, err.Error())
		return
	}

	user, err := h.authSvc.ValidateToken(c.Request.Context(), req.Token)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	ownerID := user.ID

	if req.Login != "" {
		// Если указан login, получаем документы другого пользователя
		// Здесь нужно добавить логику получения пользователя по login
		// Пока упрощено - используем текущего пользователя
		ownerID = user.ID
	}

	filter := &entities.DocumentFilter{
		OwnerID: ownerID,
		Key:     req.Key,
		Value:   req.Value,
		Limit:   req.Limit,
	}

	docs, err := h.documentSvc.GetList(c.Request.Context(), filter)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	respondWithSuccess(c, nil, dto.DocumentListResponse{Docs: docs})
}

func (h *DocumentHandler) GetByID(c *gin.Context) {
	docID := c.Param("id")
	if docID == "" {
		respondWithError(c, http.StatusBadRequest, 400, "document ID is required")
		return
	}

	token := c.Query("token")
	if token == "" {
		respondWithError(c, http.StatusBadRequest, 400, "token is required")
		return
	}

	user, err := h.authSvc.ValidateToken(c.Request.Context(), token)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	doc, err := h.documentSvc.GetByID(c.Request.Context(), docID, user.ID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	if doc.IsFile && doc.FilePath != nil {
		c.Header("Content-type", doc.MIME)
		c.Header("Content-Disposition", `attachment; filename="`+doc.Name+`"`)
		c.File(*doc.FilePath)
		return
	}

	if doc.JSONData != nil {
		var jsonData any
		if err := json.Unmarshal(*doc.JSONData, &jsonData); err == nil {
			respondWithSuccess(c, nil, jsonData)
			return
		}
	}

	respondWithSuccess(c, nil, doc)
}

func (h *DocumentHandler) Delete(c *gin.Context) {
	docID := c.Param("id")
	if docID == "" {
		respondWithError(c, http.StatusBadRequest, 400, "document ID is required")
		return
	}

	token := c.Query("token")
	if token == "" {
		respondWithError(c, http.StatusBadRequest, 400, "token is required")
		return
	}

	user, err := h.authSvc.ValidateToken(c.Request.Context(), token)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	err = h.documentSvc.Delete(c.Request.Context(), docID, user.ID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	respondWithSuccess(c, dto.DocumentDeleteResponse{ID: docID, Success: true}, nil)
}
