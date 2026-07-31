// Package contracts defines public contracts shared by Memora services.
package contracts

// Envelope is the JSON response format returned by the public API.
type Envelope struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"request_id"`
}
