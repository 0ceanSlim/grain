package server

import "time"

// Filter represents the criteria used to query events
type Filter struct {
	IDs     []string            `json:"ids"`
	Authors []string            `json:"authors"`
	Kinds   []int               `json:"kinds"`
	Tags    map[string][]string `json:"tags"`
	Since   *time.Time          `json:"since"`
	Until   *time.Time          `json:"until"`
	Limit   *int                `json:"limit"`
}
