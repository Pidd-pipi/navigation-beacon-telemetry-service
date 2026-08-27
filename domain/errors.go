package domain

import (
	"errors"
	"fmt"
	"net/http"
)

// ErrorKind 领域错误分类，决定 HTTP 状态码与前端提示。
type ErrorKind string

const (
	KindNotFound   ErrorKind = "not_found"
	KindConflict   ErrorKind = "conflict"
	KindValidation ErrorKind = "validation"
	KindDriftGuard ErrorKind = "drift_guard"
	KindInternal   ErrorKind = "internal"
)

// DomainError 携带业务分类的错误，可被 HTTP 层映射为统一响应。
type DomainError struct {
	Kind    ErrorKind
	Message string
	Cause   error
}

// Error 实现 error 接口。
func (e *DomainError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap 暴露底层错误，便于 errors.Is/As。
func (e *DomainError) Unwrap() error { return e.Cause }

// StatusCode 返回错误对应的 HTTP 状态码。
func (e *DomainError) StatusCode() int {
	switch e.Kind {
	case KindNotFound:
		return http.StatusNotFound
	case KindConflict:
		return http.StatusConflict
	case KindValidation:
		return http.StatusBadRequest
	case KindDriftGuard:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// Code 返回机器可读错误码（与 HTTP 状态码一致）。
func (e *DomainError) Code() int { return e.StatusCode() }

// NewDomainError 构造领域错误。
func NewDomainError(kind ErrorKind, message string) *DomainError {
	return &DomainError{Kind: kind, Message: message}
}

// WrapDomainError 构造带底层原因的领域错误。
func WrapDomainError(kind ErrorKind, message string, cause error) *DomainError {
	return &DomainError{Kind: kind, Message: message, Cause: cause}
}

// NotFound 资源不存在。
func NotFound(format string, args ...any) *DomainError {
	return NewDomainError(KindNotFound, fmt.Sprintf(format, args...))
}

// Conflict 状态冲突（非法状态迁移、重复回执等）。
func Conflict(format string, args ...any) *DomainError {
	return NewDomainError(KindConflict, fmt.Sprintf(format, args...))
}

// Validation 参数校验失败。
func Validation(format string, args ...any) *DomainError {
	return NewDomainError(KindValidation, fmt.Sprintf(format, args...))
}

// DriftGuard 漂移守卫拦截（漂移期间禁止关灯）。
func DriftGuard(format string, args ...any) *DomainError {
	return NewDomainError(KindDriftGuard, fmt.Sprintf(format, args...))
}

// Internal 内部错误。
func Internal(format string, args ...any) *DomainError {
	return NewDomainError(KindInternal, fmt.Sprintf(format, args...))
}

// AsDomainError 将任意 error 规整为 *DomainError，未知错误归为内部错误。
func AsDomainError(err error) *DomainError {
	if err == nil {
		return nil
	}
	var de *DomainError
	if errors.As(err, &de) {
		return de
	}
	if errors.Is(err, ErrNotFound) {
		return NewDomainError(KindNotFound, err.Error())
	}
	if errors.Is(err, ErrConflict) {
		return NewDomainError(KindConflict, err.Error())
	}
	if errors.Is(err, ErrValidation) {
		return NewDomainError(KindValidation, err.Error())
	}
	return NewDomainError(KindInternal, err.Error())
}

// 基础哨兵错误，供 errors.Is 判定。
var (
	ErrNotFound   = errors.New("resource not found")
	ErrConflict   = errors.New("state conflict")
	ErrValidation = errors.New("validation failed")
)
