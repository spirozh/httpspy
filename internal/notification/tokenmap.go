// Package notification provides a notification channel service
package notification

import (
	"encoding/json"
	"math/rand"
	"sync"

	"www.github.com/spirozh/httpspy/internal/data"
)

// Token is returned with a notification channel, so that the channel can be closed.
type Token int

// Nothing is the empty struct
type Nothing struct{}

var mu sync.RWMutex
var tokenMap map[Token]chan data.SSEEvent

// New returns a new notification channel and a token that can be used to close it.
func New() (<-chan data.SSEEvent, Token) {
	mu.Lock()
	defer mu.Unlock()

	if tokenMap == nil {
		tokenMap = map[Token]chan data.SSEEvent{}
	}

	ch := make(chan data.SSEEvent, 1)

	for {
		tok := Token(rand.Int())
		if _, exists := tokenMap[tok]; !exists {
			tokenMap[tok] = ch
			return ch, tok
		}
	}
}

// Close closes a notification channel
func Close(tok Token) {
	mu.Lock()
	defer mu.Unlock()

	ch := tokenMap[tok]
	if ch != nil {
		close(ch)
		delete(tokenMap, tok)
	}
}

// NotifyClear sends a clear to all open notification channels
func NotifyClear() {
	sse := data.SSEEvent{
		Event: "clear",
		Data:  []byte("[]"),
	}
	notify(sse)
}

// NotifyNew sends a new request to all open notification channels
func NotifyNew(request data.Request) {
	marshalledRequest, _ := json.Marshal(request)
	sse := data.SSEEvent{
		Event: "new",
		Data:  marshalledRequest,
	}

	notify(sse)
}

func notify(sse data.SSEEvent) {
	mu.RLock()
	defer mu.RUnlock()

	for _, ch := range tokenMap {
		ch <- sse
	}
}
