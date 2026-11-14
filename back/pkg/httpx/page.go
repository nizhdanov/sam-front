package httpx

import "net/http"

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

type PageRequest struct {
	Page  int
	Limit int
}

func (p PageRequest) Offset() int { return (p.Page - 1) * p.Limit }

type Pagination struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

type Page[T any] struct {
	Items      []T        `json:"items"`
	Pagination Pagination `json:"pagination"`
}

func NewPage[T any](items []T, req PageRequest, total int) Page[T] {
	if items == nil {
		items = []T{}
	}
	pages := 0
	if req.Limit > 0 {
		pages = (total + req.Limit - 1) / req.Limit
	}
	return Page[T]{Items: items, Pagination: Pagination{Page: req.Page, Limit: req.Limit, Total: total, TotalPages: pages}}
}

func ParsePage(r *http.Request) (PageRequest, error) {
	page, err := QueryInt(r, "page", 1, 1, 1_000_000)
	if err != nil {
		return PageRequest{}, err
	}
	limit, err := QueryInt(r, "limit", DefaultLimit, 1, MaxLimit)
	if err != nil {
		return PageRequest{}, err
	}
	return PageRequest{Page: page, Limit: limit}, nil
}
