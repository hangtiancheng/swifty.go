package single_flight

import "sync"

type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

// Group suppresses duplicate in-flight calls for the same key.
type Group struct {
	m sync.Map
}

// Do runs fn once for a key while concurrent duplicate callers wait for the same result.
func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	c := &call{}
	c.wg.Add(1)

	actual, loaded := g.m.LoadOrStore(key, c)
	if loaded {
		existing := actual.(*call)
		existing.wg.Wait()
		return existing.val, existing.err
	}

	defer g.m.Delete(key)
	defer c.wg.Done()

	c.val, c.err = fn()
	return c.val, c.err
}
