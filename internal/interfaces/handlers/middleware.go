package handlers

import (
	"document-server/internal/interfaces/dto"
	"document-server/pkg/errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

func respondWithError(c *gin.Context, httpStatus, errorCode int, message string) {
	c.JSON(httpStatus, dto.APIResponse{
		Error: &dto.ErrorResponse{
			Code: errorCode,
			Text: message,
		},
	})
}

func respondWithSuccess(c *gin.Context, response, data any) {
	c.JSON(http.StatusOK, dto.APIResponse{
		Response: response,
		Data:     data,
	})
}

func handleServiceError(c *gin.Context, err error) {
	switch e := err.(type) {
	case *errors.BadRequestError:
		respondWithError(c, http.StatusBadRequest, 400, e.Message)
	case *errors.UnauthorizedError:
		respondWithError(c, http.StatusUnauthorized, 401, e.Message)
	case *errors.ForbiddenError:
		respondWithError(c, http.StatusForbidden, 403, e.Message)
	case *errors.NotFoundError:
		respondWithError(c, http.StatusNotFound, 404, e.Message)
	case *errors.InternalError:
		respondWithError(c, http.StatusInternalServerError, 500, e.Message)
	default:
		respondWithError(c, http.StatusInternalServerError, 500, "internal server error")
	}
}

func CORSMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})
}

func HeadToGetMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		if c.Request.Method == "HEAD" {
			c.Request.Method = "GET"
			c.Writer = &headResponseWriter{c.Writer}
		}
		c.Next()
	})
}

type headResponseWriter struct {
	gin.ResponseWriter
}

func (w *headResponseWriter) Write(data []byte) (int, error) {
	return len(data), nil
}
