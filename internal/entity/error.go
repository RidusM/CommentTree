package entity

import "errors"

var (
	ErrCommentNotFound  = errors.New("comment not found")
	ErrCommentDeleted   = errors.New("comment deleted")
	ErrParentNotFound   = errors.New("parent comment not found")
	ErrMaxDepthExceeded = errors.New("maximum nesting depth exceeded")
	ErrInvalidData      = errors.New("invalid data")
	ErrConflictingData  = errors.New("conflicting data")
)
