package objects

import (
	"time"
)

type ModelIdentify struct {
	ID string `json:"id"`
}

type Model struct {
	ID string `json:"id"`
	// Display name, for user-friendly display from anthropic API.
	DisplayName string `json:"display_name"`
	// Created time in seconds.
	Created int64 `json:"created"`
	// Created time in time.Time.
	CreatedAt time.Time `json:"created_at"`
	// Owned by
	OwnedBy string `json:"owned_by"`
}
