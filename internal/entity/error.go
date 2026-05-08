package entity

import "errors"

var (
	ErrDataNotFound     = errors.New("data not found")
	ErrConflictingData  = errors.New("conflicting data")
	ErrInvalidData      = errors.New("invalid data")
	ErrCommentNotFound  = errors.New("comment not found")
	ErrParentNotFound   = errors.New("parent comment not found")
	ErrMaxDepthExceeded = errors.New("maximum nesting depth exceeded")
)
