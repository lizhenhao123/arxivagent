package service

import "errors"

var (
	ErrNotFound     = errors.New("resource not found")
	ErrInvalidState = errors.New("invalid state transition")
	ErrValidation   = errors.New("validation failed")
)
