package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/innoscripta-banking-ledger/internal/api_gateway/middleware"
)

// Response represents a standard API response
type Response struct {
	Data          interface{} `json:"data,omitempty"`
	Error         *ErrorInfo  `json:"error,omitempty"`
	CorrelationID string      `json:"correlation_id,omitempty"`
	Meta          *MetaInfo   `json:"meta,omitempty"`
}

// ErrorInfo represents error information in a response
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// MetaInfo represents metadata in a response
type MetaInfo struct {
	Page       int `json:"page,omitempty"`
	PerPage    int `json:"per_page,omitempty"`
	TotalPages int `json:"total_pages,omitempty"`
	TotalItems int `json:"total_items,omitempty"`
}

// NewResponse creates a new response with data
func NewResponse(data interface{}) *Response {
	return &Response{
		Data: data,
	}
}

// NewErrorResponse creates a new error response
func NewErrorResponse(code, message string) *Response {
	return &Response{
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
		},
	}
}

// NewPaginatedResponse creates a new paginated response
func NewPaginatedResponse(data interface{}, page, perPage, totalItems int) *Response {
	totalPages := totalItems / perPage
	if totalItems%perPage > 0 {
		totalPages++
	}

	return &Response{
		Data: data,
		Meta: &MetaInfo{
			Page:       page,
			PerPage:    perPage,
			TotalPages: totalPages,
			TotalItems: totalItems,
		},
	}
}

// RespondWithData sends a JSON response with data
func RespondWithData(c *gin.Context, statusCode int, data interface{}) {
	response := NewResponse(data)
	response.CorrelationID = middleware.GetCorrelationID(c)
	c.JSON(statusCode, response)
}

// RespondWithError sends a JSON response with an error
func RespondWithError(c *gin.Context, statusCode int, code, message string) {
	response := NewErrorResponse(code, message)
	response.CorrelationID = middleware.GetCorrelationID(c)
	c.JSON(statusCode, response)
}

// RespondWithPaginatedData sends a JSON response with paginated data
func RespondWithPaginatedData(c *gin.Context, statusCode int, data interface{}, page, perPage, totalItems int) {
	response := NewPaginatedResponse(data, page, perPage, totalItems)
	response.CorrelationID = middleware.GetCorrelationID(c)
	c.JSON(statusCode, response)
}

// RespondOK sends a 200 OK response with data
func RespondOK(c *gin.Context, data interface{}) {
	RespondWithData(c, http.StatusOK, data)
}

// RespondCreated sends a 201 Created response with data
func RespondCreated(c *gin.Context, data interface{}) {
	RespondWithData(c, http.StatusCreated, data)
}

// RespondAccepted sends a 202 Accepted response with data.
func RespondAccepted(c *gin.Context, data interface{}) {
	RespondWithData(c, http.StatusAccepted, data)
}

// RespondNoContent sends a 204 No Content response
func RespondNoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// RespondBadRequest sends a 400 Bad Request response with an error
func RespondBadRequest(c *gin.Context, message string) {
	RespondWithError(c, http.StatusBadRequest, "BAD_REQUEST", message)
}

// RespondUnauthorized sends a 401 Unauthorized response with an error
func RespondUnauthorized(c *gin.Context, message string) {
	if message == "" {
		message = "Unauthorized"
	}
	RespondWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", message)
}

// RespondForbidden sends a 403 Forbidden response with an error
func RespondForbidden(c *gin.Context, message string) {
	if message == "" {
		message = "Forbidden"
	}
	RespondWithError(c, http.StatusForbidden, "FORBIDDEN", message)
}

// RespondNotFound sends a 404 Not Found response with an error
func RespondNotFound(c *gin.Context, message string) {
	if message == "" {
		message = "Resource not found"
	}
	RespondWithError(c, http.StatusNotFound, "NOT_FOUND", message)
}

// RespondConflict sends a 409 Conflict response with an error
func RespondConflict(c *gin.Context, message string) {
	RespondWithError(c, http.StatusConflict, "CONFLICT", message)
}

// RespondInternalError sends a 500 Internal Server Error response with an error
func RespondInternalError(c *gin.Context) {
	RespondWithError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "An internal server error occurred")
}
