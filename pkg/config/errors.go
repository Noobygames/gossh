package config

import "fmt"

// ParseError is returned when a config file exists but contains invalid YAML.
type ParseError struct {
	Path string
	Err  error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse config %s: %v", e.Path, e.Err)
}

func (e *ParseError) Unwrap() error { return e.Err }
