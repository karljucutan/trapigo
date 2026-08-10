package domain

import (
	"net/url"
	"sync"
	"sync/atomic"
)

type Backend struct {
	Id  string
	URL *url.URL
}

type BackendPool struct {
	Backends []*Backend
	Current  int64
	mu       sync.Mutex
}

func NewBackend(id string, url *url.URL) *Backend {
	return &Backend{
		Id:  id,
		URL: url,
	}
}

func NewBackendPool() *BackendPool {
	return &BackendPool{
		Backends: []*Backend{},
	}
}

func (bp *BackendPool) Add(backend *Backend) {
	bp.Backends = append(bp.Backends, backend)
}

func (p *BackendPool) RoundRobinMutex() *Backend {
	if p == nil || len(p.Backends) == 0 {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	backendLength := int64(len(p.Backends))
	backend := p.Backends[p.Current%backendLength]
	p.Current++
	return backend
}

func (p *BackendPool) RoundRobinAtomic() *Backend {
	if p == nil || len(p.Backends) == 0 {
		return nil
	}

	// Safely increments and returns the new value in 1 CPU step (no mutex needed)
	newCurrent := atomic.AddInt64(&p.Current, 1)
	currentIndex := newCurrent - 1
	backendLength := int64(len(p.Backends))
	backend := p.Backends[currentIndex%backendLength]
	return backend
}
