// Package httpapi 提供 REST API 路由与处理器，统一响应格式与错误映射。
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"example.com/navigation-beacon-telemetry-service/domain"
)

// maxBodyBytes 请求体上限（1 MiB），超过则返回 413。
const maxBodyBytes = 1 << 20

// 分页默认值与上限。
const (
	defaultPageLimit = 20
	maxPageLimit     = 1000
)

// Response 统一响应格式：code=0 表示成功，非 0 为错误码（与 HTTP 状态码一致）。
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	Total   int    `json:"total,omitempty"` // 列表接口命中过滤条件的总条数
}

// writeJSON 输出 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// OK 输出 200 成功响应。
func OK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, Response{Code: 0, Message: "ok", Data: data})
}

// OKPage 输出带 total 的 200 分页成功响应。
func OKPage(w http.ResponseWriter, data any, total int) {
	writeJSON(w, http.StatusOK, Response{Code: 0, Message: "ok", Data: data, Total: total})
}

// Created 输出 201 创建成功响应。
func Created(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusCreated, Response{Code: 0, Message: "created", Data: data})
}

// Fail 将领域错误映射为统一错误响应。
func Fail(w http.ResponseWriter, err error) {
	de := domain.AsDomainError(err)
	writeJSON(w, de.StatusCode(), Response{Code: de.Code(), Message: de.Message})
}

// invalidBody 输出请求体解析错误；过大请求体返回 413，其余返回 400。
func invalidBody(w http.ResponseWriter, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		writeJSON(w, http.StatusRequestEntityTooLarge, Response{
			Code:    http.StatusRequestEntityTooLarge,
			Message: fmt.Sprintf("请求体过大，最大允许 %d 字节", maxBodyBytes),
		})
		return
	}
	Fail(w, domain.Validation("请求体解析失败: %v", err))
}

// decodeJSON 严格解析请求体 JSON：限制大小、拒绝 NaN/Inf 溢出、拒绝尾随 JSON。
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("请求体包含多个 JSON 值")
		}
		return err
	}
	return nil
}

// pathID 读取路由通配符 {id}。
func pathID(r *http.Request) string {
	return r.PathValue("id")
}

// pageParams 分页参数。
type pageParams struct {
	Limit  int
	Offset int
}

// parsePagination 解析并校验 limit/offset。limit 为 0 时使用默认值，
// 超过上限时截断到上限；非法或负值返回 400 校验错误。
func parsePagination(r *http.Request) (pageParams, error) {
	q := r.URL.Query()
	limit := defaultPageLimit
	offset := 0
	var err error
	if v := q.Get("limit"); v != "" {
		if limit, err = strconv.Atoi(v); err != nil || limit < 0 {
			return pageParams{}, domain.Validation("limit 必须为非负整数")
		}
	}
	if v := q.Get("offset"); v != "" {
		if offset, err = strconv.Atoi(v); err != nil || offset < 0 {
			return pageParams{}, domain.Validation("offset 必须为非负整数")
		}
	}
	if limit == 0 {
		limit = defaultPageLimit
	}
	if limit > maxPageLimit {
		limit = maxPageLimit
	}
	return pageParams{Limit: limit, Offset: offset}, nil
}

// paginate 对已完成过滤排序的切片做内存分页。
func paginate[T any](items []T, page pageParams) []T {
	if page.Offset >= len(items) {
		return []T{}
	}
	end := page.Offset + page.Limit
	if end > len(items) {
		end = len(items)
	}
	return items[page.Offset:end]
}
