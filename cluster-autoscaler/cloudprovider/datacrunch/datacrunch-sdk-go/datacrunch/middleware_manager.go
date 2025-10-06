package datacrunch

import (
	"sync"
)

// Middleware manages request and response middleware chains with thread-safe operations
type Middleware struct {
	mu                 sync.RWMutex
	requestMiddleware  []RequestMiddleware
	responseMiddleware []ResponseMiddleware
}

// NewMiddleware creates a new Middleware manager with optional default middleware
func NewMiddleware(requestMiddleware []RequestMiddleware, responseMiddleware []ResponseMiddleware) *Middleware {
	return &Middleware{
		requestMiddleware:  append([]RequestMiddleware{}, requestMiddleware...),
		responseMiddleware: append([]ResponseMiddleware{}, responseMiddleware...),
	}
}

// NewDefaultMiddleware creates a Middleware manager with the standard default middleware
func NewDefaultMiddleware(logger Logger) *Middleware {
	requestMiddleware := []RequestMiddleware{
		AuthenticationMiddleware(),
		JSONContentTypeMiddleware(),
	}

	responseMiddleware := []ResponseMiddleware{
		ErrorHandlingMiddleware(),
	}

	// Add logging middleware if logger supports debug (not NoOpLogger)
	if _, isNoOp := logger.(*NoOpLogger); !isNoOp {
		requestMiddleware = append(requestMiddleware, LoggingMiddleware(logger))
		responseMiddleware = append(responseMiddleware, ResponseLoggingMiddleware(logger))
	}

	return NewMiddleware(requestMiddleware, responseMiddleware)
}

// Snapshot returns thread-safe copies of the current middleware chains
func (m *Middleware) Snapshot() ([]RequestMiddleware, []ResponseMiddleware) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	requestCopy := append([]RequestMiddleware{}, m.requestMiddleware...)
	responseCopy := append([]ResponseMiddleware{}, m.responseMiddleware...)

	return requestCopy, responseCopy
}

// Request middleware management
func (m *Middleware) AddRequestMiddleware(middleware RequestMiddleware) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestMiddleware = append(m.requestMiddleware, middleware)
}

func (m *Middleware) SetRequestMiddleware(middleware []RequestMiddleware) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestMiddleware = append([]RequestMiddleware{}, middleware...)
}

func (m *Middleware) ClearRequestMiddleware() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestMiddleware = []RequestMiddleware{}
}

func (m *Middleware) LenRequestMiddleware() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.requestMiddleware)
}

// Response middleware management
func (m *Middleware) AddResponseMiddleware(middleware ResponseMiddleware) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responseMiddleware = append(m.responseMiddleware, middleware)
}

func (m *Middleware) SetResponseMiddleware(middleware []ResponseMiddleware) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responseMiddleware = append([]ResponseMiddleware{}, middleware...)
}

func (m *Middleware) ClearResponseMiddleware() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responseMiddleware = []ResponseMiddleware{}
}

func (m *Middleware) LenResponseMiddleware() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.responseMiddleware)
}

// Convenience methods for bulk operations
func (m *Middleware) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestMiddleware = []RequestMiddleware{}
	m.responseMiddleware = []ResponseMiddleware{}
}

func (m *Middleware) Len() (requestCount, responseCount int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.requestMiddleware), len(m.responseMiddleware)
}
