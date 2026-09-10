// Package example provides a sample Object implementation for use with the
// consistent_cache service.
package example

import (
	"encoding/json"
)

// Example is a sample data record backed by the `example` MySQL table.
type Example struct {
	ID   uint   `json:"id" gorm:"primarykey"`
	Key_ string `json:"key" gorm:"column:key"`
	Data string `json:"data" gorm:"column:data"`
}

// TableName returns the corresponding table name.
func (e *Example) TableName() string {
	return "example"
}

// KeyColumn returns the column name of the key.
func (e *Example) KeyColumn() string {
	return "key"
}

// Key returns the value of the key.
func (e *Example) Key() string {
	return e.Key_
}

// DataColumn returns the columns holding the data payload.
func (e *Example) DataColumn() []string {
	return []string{"data"}
}

// Write serializes the object into a string.
func (e *Example) Write() (string, error) {
	body, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// Read deserializes the string body into the object instance.
func (e *Example) Read(body string) error {
	return json.Unmarshal([]byte(body), e)
}
