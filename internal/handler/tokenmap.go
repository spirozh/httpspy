package handler

import (
	"math/rand"
	"sync"
)

type token int

// TokenChanMap manages tokens associated with channels
type TokenChanMap struct {
	mu sync.RWMutex
	m  map[token]chan nothing
}

type nothing struct{}

// NewTokenChanMap creates a new TokenChanMap
func NewTokenChanMap() *TokenChanMap {
	return &TokenChanMap{
		m: map[token]chan nothing{},
	}
}

func (m *TokenChanMap) New() (token, <-chan nothing) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan nothing, 1)
	ch <- nothing{}

	for {
		tok := token(rand.Int())
		if _, exists := m.m[tok]; !exists {
			m.m[tok] = ch
			return tok, ch
		}
	}
}

func (m *TokenChanMap) Close(tok token) {
	m.mu.Lock()
	defer m.mu.Unlock()

	close(m.m[tok])
	delete(m.m, tok)
}

func (m *TokenChanMap) Notify() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for tok := range m.m {
		m.m[tok] <- nothing{}
	}
}
