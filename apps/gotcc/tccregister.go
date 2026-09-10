package gotcc

import (
	"errors"
	"fmt"
	"sync"
)

// registryCenter keeps track of every TCCComponent registered with a
// TXManager, keyed by component id.
type registryCenter struct {
	mux        sync.RWMutex
	components map[string]TCCComponent
}

func newRegistryCenter() *registryCenter {
	return &registryCenter{
		components: make(map[string]TCCComponent),
	}
}

func (r *registryCenter) register(component TCCComponent) error {
	r.mux.Lock()
	defer r.mux.Unlock()
	if _, ok := r.components[component.ID()]; ok {
		return fmt.Errorf("duplicate component id: %s", component.ID())
	}
	r.components[component.ID()] = component
	return nil
}

func (r *registryCenter) getComponents(componentIDs ...string) ([]TCCComponent, error) {
	if len(componentIDs) == 0 {
		return nil, errNoComponentIDs
	}

	components := make([]TCCComponent, 0, len(componentIDs))

	r.mux.RLock()
	defer r.mux.RUnlock()

	for _, componentID := range componentIDs {
		component, ok := r.components[componentID]
		if !ok {
			return nil, fmt.Errorf("component id: %s not registered", componentID)
		}
		components = append(components, component)
	}

	return components, nil
}

// errNoComponentIDs is returned when an empty id list is requested.
var errNoComponentIDs = errors.New("no component ids given")
