// Package output owns versioned automation output contracts.
package output

import "time"

const SchemaVersion = 1

type Envelope struct {
	SchemaVersion int       `json:"schema_version"`
	Command       string    `json:"command"`
	GeneratedAt   time.Time `json:"generated_at"`
	Data          any       `json:"data"`
	Warnings      []string  `json:"warnings"`
	Partial       bool      `json:"partial"`
	Error         *Error    `json:"error,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewError(command, code, message string, now time.Time) Envelope {
	envelope := New(command, nil, now)
	envelope.Error = &Error{Code: code, Message: message}
	return envelope
}

func New(command string, data any, now time.Time) Envelope {
	return Envelope{
		SchemaVersion: SchemaVersion,
		Command:       command,
		GeneratedAt:   now.UTC(),
		Data:          data,
		Warnings:      []string{},
	}
}
