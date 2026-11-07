package service

import (
	"errors"
	"sync"
)

// operationGate reserves one package/deployment writer, including preparation.
// It is process-local; it does not lock session files against another process.
type operationGate struct {
	mu     sync.Mutex
	busy   bool
	closed bool
}

func (g *operationGate) acquire() (func(), error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return nil, errors.New("Приложение завершает работу.")
	}
	if g.busy {
		return nil, errors.New("Уже выполняется установка или развёртывание. Дождитесь завершения операции.")
	}
	g.busy = true
	var once sync.Once
	return func() { once.Do(func() { g.mu.Lock(); g.busy = false; g.mu.Unlock() }) }, nil
}

func (g *operationGate) close() {
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
}
