// Package notification provides a notification channel service
package notification

import (
	"math/rand"
	"sync"
)

// Token is returned with a notification channel, so that the channel can be closed.
type Token int

// Nothing is the empty struct
type Nothing struct{}

var mu sync.RWMutex
var tokenMap map[Token]chan Nothing

// New returns a new notification channel and a token that can be used to close it.
func New() (<-chan Nothing, Token) {
	mu.Lock()
	defer mu.Unlock()

	if tokenMap == nil {
		tokenMap = map[Token]chan Nothing{}
	}

	ch := make(chan Nothing, 1)
	ch <- Nothing{}

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
		ch <- Nothing{}
	}
}
