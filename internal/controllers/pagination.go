package controllers

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"ssd/internal/models"
)

const maxLimit = 10000

type paginatedResponse struct {
	Data   any `json:"data"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

func parsePagination(r *http.Request) (limit, offset int) {
	q := r.URL.Query()
	if v := q.Get("limit"); v != "" {
		limit, _ = strconv.Atoi(v)
		if limit < 0 {
			limit = 0
		}
		if limit > maxLimit {
			limit = maxLimit
		}
	}
	if v := q.Get("offset"); v != "" {
		offset, _ = strconv.Atoi(v)
		if offset < 0 {
			offset = 0
		}
	}
	return
}

func hasPagination(limit, offset int) bool {
	return limit > 0 || offset > 0
}

func paginatedCacheKey(base string, limit, offset int) string {
	if !hasPagination(limit, offset) {
		return base
	}
	return fmt.Sprintf("%s:%d:%d", base, limit, offset)
}

func paginateIntMap(data map[int]*models.StatRecord, limit, offset int) (map[int]*models.StatRecord, int) {
	if data == nil {
		return nil, 0
	}
	total := len(data)

	keys := make([]int, 0, total)
	for k := range data {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	if offset >= len(keys) {
		return make(map[int]*models.StatRecord), total
	}
	keys = keys[offset:]
	if limit > 0 && limit < len(keys) {
		keys = keys[:limit]
	}

	result := make(map[int]*models.StatRecord, len(keys))
	for _, k := range keys {
		result[k] = data[k]
	}
	return result, total
}

func paginateStringMap(data map[string]*models.Statistic, limit, offset int) (map[string]*models.Statistic, int) {
	if data == nil {
		return nil, 0
	}
	total := len(data)

	keys := make([]string, 0, total)
	for k := range data {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	if offset >= len(keys) {
		return make(map[string]*models.Statistic), total
	}
	keys = keys[offset:]
	if limit > 0 && limit < len(keys) {
		keys = keys[:limit]
	}

	result := make(map[string]*models.Statistic, len(keys))
	for _, k := range keys {
		result[k] = data[k]
	}
	return result, total
}

func paginateSlice(data []string, limit, offset int) ([]string, int) {
	if data == nil {
		return nil, 0
	}
	total := len(data)

	if offset >= len(data) {
		return []string{}, total
	}
	data = data[offset:]
	if limit > 0 && limit < len(data) {
		data = data[:limit]
	}
	return data, total
}
