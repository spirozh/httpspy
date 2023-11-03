package httpspy

import (
	"fmt"
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

	idChan      chan int64
	dbErrorChan chan error
}

func (r Request) String() string {
	return fmt.Sprintf(`
	id: %d
	timestamp: %s
	method url: %s %s,
	header: %v,
	body: %q
	`, r.ID, r.Timestamp, r.Method, r.URL, r.Headers, r.Body)
}
