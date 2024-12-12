package model

import "time"

// Rule godoc
// @Description Represents a custom rule for a domain
// @Type Rule
type Rule struct {
	ID        int       `json:"id"`
	Domain    string    `json:"domain"`
	RobotsTxt string    `json:"robots_txt"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
