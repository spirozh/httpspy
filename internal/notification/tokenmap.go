package notification

import (
	"math/rand"
	"sync"
)

// Token is returned with a notification channel, so that the channel can be closed.
type Token int
type nothing struct{}

var mu sync.RWMutex
var tokenMap map[Token]chan nothing

// Returns a new notification channel and a token that can be used to close it.
func New() (<-chan nothing, Token) {
	mu.Lock()
	defer mu.Unlock()

	ch := make(chan nothing, 1)
	ch <- nothing{}

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

// Notify sends something to all open notification channels
func Notify() {
	mu.RLock()
	defer mu.RUnlock()

	for _, ch := range tokenMap {
		ch <- nothing{}
	}
}
