package main

import (
	"math/rand"
	"sync"
)

type token int

type tokenChanMap struct {
	mu sync.RWMutex
	m  map[token]chan struct{}
}

func newTokenChanMap() *tokenChanMap {
	return &tokenChanMap{
		m: map[token]chan struct{}{},
	}
}

func (m *tokenChanMap) newToken() (token, <-chan struct{}) {
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

func (m *tokenChanMap) close(tok token) {
	m.mu.Lock()
	defer m.mu.Unlock()

	close(m.m[tok])
	delete(m.m, tok)
}

func (m *tokenChanMap) notify() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for tok := range m.m {
		m.m[tok] <- struct{}{}
	}
}
