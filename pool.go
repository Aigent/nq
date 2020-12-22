package nq

import "sync"

// sync.Pool cleans itself too aggressively: every 2nd GC cycle or so
// This is very simple pool with long living items
type pool struct {
	items []interface{}
	mu    sync.Mutex
	New   func() interface{}
}

// Get returns one of the available items or try to creates new one
func (p *pool) Get() interface{} {
	var item interface{}

	p.mu.Lock()
	if len(p.items) > 0 {
		last := len(p.items) - 1
		item = p.items[last]
		p.items = p.items[:last]
	}
	p.mu.Unlock()

	if item != nil {
		return item
	}
	if p.New != nil {
		return p.New()
	}
	return nil
}

// Put adds an element to the pool for later reuse
func (p *pool) Put(item interface{}) {
	p.mu.Lock()
	p.items = append(p.items, item)
	p.mu.Unlock()
}
