package proposals

import "sync"

type SafeCounter struct {
	sync.RWMutex
	count int
}

// Increment safely increments the counter by 1
func (c *SafeCounter) Increment() {
	c.Lock()
	defer c.Unlock()
	c.count++
}

// Decrement safely decrements the counter by 1
func (c *SafeCounter) Decrement() {
	c.Lock()
	defer c.Unlock()
	c.count--
}

// GetValue safely retrieves the current value of the counter
func (c *SafeCounter) GetValue() int {
	c.RLock()
	defer c.RUnlock()
	return c.count
}

func (c *SafeCounter) Reset() {
	c.Lock()
	defer c.Unlock()
	c.count = 0
}

func (c *SafeCounter) Set(val int) {
	c.Lock()
	defer c.Unlock()
	c.count = val
}
