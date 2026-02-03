package entity

import "errors"

var (
	ErrDataNotFound     = errors.New("data not found")
	ErrInvalidData      = errors.New("invalid data")
	ErrConflictingData  = errors.New("conflicting data")
	ErrCommentNotFound  = errors.New("comment not found")
	ErrCommentDeleted   = errors.New("comment deleted")
	ErrParentNotFound   = errors.New("parent comment not found")
	ErrMaxDepthExceeded = errors.New("maximum nesting depth exceeded")
	ErrInvalidParent    = errors.New("invalid parent comment")
)
