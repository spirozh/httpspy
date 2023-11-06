// Package data contains the data types used in the application
package data

import (
	"time"
)

// Request contains information about an incoming request, plus two channels for async updates
type Request struct {
	ID        int64
	Timestamp time.Time
	Method    string
	URL       string
	Headers   string
	Body      string
}

type SSEEvent struct {
	Event string
	Data  []byte
}
