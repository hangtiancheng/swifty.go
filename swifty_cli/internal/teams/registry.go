package teams

import "sync"

// NameRegistry is the process-wide global name registry, maintaining
// member name → agentID mappings. SendMessage uses it to resolve the recipient
// name given by the user/LLM into a delivery identifier.
type NameRegistry struct {
	mu    sync.Mutex
	names map[string]string // name -> agentID
}

var (
	nameRegistryOnce     sync.Once
	nameRegistryInstance *NameRegistry
)

// GetNameRegistry returns the global singleton registry.
func GetNameRegistry() *NameRegistry {
	nameRegistryOnce.Do(func() {
		nameRegistryInstance = &NameRegistry{names: make(map[string]string)}
	})
	return nameRegistryInstance
}

// Register records a name → agentID mapping.
func (r *NameRegistry) Register(name, agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.names[name] = agentID
}

// Resolve resolves a name or ID into an agentID: it looks up the name first;
// if the input is itself an ID, it is returned as-is; otherwise an empty
// string is returned.
func (r *NameRegistry) Resolve(nameOrID string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id, ok := r.names[nameOrID]; ok {
		return id
	}
	for _, id := range r.names {
		if id == nameOrID {
			return nameOrID
		}
	}
	return ""
}

// Unregister removes the mapping for a name.
func (r *NameRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.names, name)
}

// Clear removes all mappings; mainly used for test isolation.
func (r *NameRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.names = make(map[string]string)
}
