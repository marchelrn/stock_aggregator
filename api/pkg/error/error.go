package errs

import (
	"fmt"
	"net/http"
)

type AppError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func (e *AppError) Error() string {
	return e.Message
}

// BadRequest - 400
func BadRequest(message string) *AppError {
	return &AppError{
		Code:    http.StatusBadRequest,
		Message: message,
	}
}

// Unauthorized - 401
func Unauthorized(message string) *AppError {
	return &AppError{
		Code:    http.StatusUnauthorized,
		Message: message,
	}
}

// Forbidden - 403
func Forbidden(message string) *AppError {
	return &AppError{
		Code:    http.StatusForbidden,
		Message: message,
	}
}

// NotFound - 404
func NotFound(message string) *AppError {
	return &AppError{
		Code:    http.StatusNotFound,
		Message: message,
	}
}

// Conflict - 409 (duplicate data)
func Conflict(message string) *AppError {
	return &AppError{
		Code:    http.StatusConflict,
		Message: message,
	}
}

// UnprocessableEntity - 422 (validation error)
func UnprocessableEntity(message string) *AppError {
	return &AppError{
		Code:    http.StatusUnprocessableEntity,
		Message: message,
	}
}

// TooManyRequests - 429
func TooManyRequests(message string) *AppError {
	return &AppError{
		Code:    http.StatusTooManyRequests,
		Message: message,
	}
}

// InternalServerError - 500
func InternalServerError(message string) *AppError {
	return &AppError{
		Code:    http.StatusInternalServerError,
		Message: message,
	}
}

func Wrap(err error, code int, message string) *AppError {
	if message == "" {
		message = err.Error()
	}
	return &AppError{
		Code:    code,
		Message: fmt.Sprintf("%s: %v", message, err),
	}
}

func GetStatusCode(err error) int {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code
	}
	return http.StatusInternalServerError
}

func IsAppError(err error) bool {
	_, ok := err.(*AppError)
	return ok
}
