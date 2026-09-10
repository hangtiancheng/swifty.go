package gotcc

import (
	"time"
)

// RequestEntity binds a business request to the component that should
// execute it.
type RequestEntity struct {
	// ComponentID is the target component identifier.
	ComponentID string `json:"componentName"`
	// Request carries the component specific business input.
	Request map[string]interface{} `json:"request"`
}

// ComponentEntities is a slice of ComponentEntity.
type ComponentEntities []*ComponentEntity

// ToComponents extracts the bare TCCComponent list from the entities.
func (c ComponentEntities) ToComponents() []TCCComponent {
	components := make([]TCCComponent, 0, len(c))
	for _, entity := range c {
		components = append(components, entity.Component)
	}
	return components
}

// ComponentEntity pairs a component with the request it should be invoked
// with.
type ComponentEntity struct {
	Request   map[string]interface{}
	Component TCCComponent
}

// TXStatus is the state of a whole transaction.
type TXStatus string

// String implements fmt.Stringer.
func (t TXStatus) String() string {
	return string(t)
}

const (
	// TXHanging means the transaction is still in progress.
	TXHanging TXStatus = "hanging"
	// TXSuccessful means every component confirmed.
	TXSuccessful TXStatus = "successful"
	// TXFailure means the transaction was rolled back.
	TXFailure TXStatus = "failure"
)

// ComponentTryStatus is the try-phase state of a single component.
type ComponentTryStatus string

// String implements fmt.Stringer.
func (c ComponentTryStatus) String() string {
	return string(c)
}

const (
	// TryHanging means the try operation has not reported back yet.
	TryHanging ComponentTryStatus = "hanging"
	// TrySuccessful means the try operation was accepted.
	TrySuccessful ComponentTryStatus = "successful"
	// TryFailure means the try operation was rejected or failed.
	TryFailure ComponentTryStatus = "failure"
)

// ComponentTryEntity is one component's try-phase state inside a
// transaction.
type ComponentTryEntity struct {
	ComponentID string
	TryStatus   ComponentTryStatus
}

// Transaction is the log entry describing one distributed transaction.
type Transaction struct {
	TXID       string                `json:"txID"`
	Components []*ComponentTryEntity `json:"components"`
	Status     TXStatus              `json:"status"`
	CreatedAt  time.Time             `json:"createdAt"`
}

// getStatus infers the current transaction state from the try results of
// its components. createdBefore marks the point in time before which a
// still hanging transaction is considered timed out.
func (t *Transaction) getStatus(createdBefore time.Time) TXStatus {
	// 1. Any failed component fails the whole transaction.
	var hangingExist bool
	for _, component := range t.Components {
		if component.TryStatus == TryFailure {
			return TXFailure
		}
		hangingExist = hangingExist || (component.TryStatus != TrySuccessful)
	}

	// 2. A transaction that is still hanging past its deadline fails.
	if hangingExist && t.CreatedAt.Before(createdBefore) {
		return TXFailure
	}

	// 3. If any component is still hanging, the transaction is hanging.
	if hangingExist {
		return TXHanging
	}

	// 4. Otherwise every component's try phase succeeded.
	return TXSuccessful
}
