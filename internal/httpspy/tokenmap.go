package httpspy

import (
	"math/rand"
	"sync"
)

type token int

// TokenChanMap manages tokens associated with channels
type TokenChanMap struct {
	mu sync.RWMutex
	m  map[token]chan struct{}
}

// NewTokenChanMap creates a new TokenChanMap
func NewTokenChanMap() *TokenChanMap {
	return &TokenChanMap{
		m: map[token]chan struct{}{},
	}
}

func (m *TokenChanMap) newToken() (token, <-chan struct{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan struct{}, 1)
	ch <- struct{}{}

	for {
		tok := token(rand.Int())
		if _, exists := m.m[tok]; !exists {
			m.m[tok] = ch
			return tok, ch
		}
	}
}

func (m *TokenChanMap) close(tok token) {
	m.mu.Lock()
	defer m.mu.Unlock()

	close(m.m[tok])
	delete(m.m, tok)
}

func (m *TokenChanMap) notify() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for tok := range m.m {
		m.m[tok] <- struct{}{}
	}
}
