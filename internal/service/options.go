package service

type Option func(*CommentService)

func WithMaxDepth(depth int) Option {
	return func(s *CommentService) {
		s.maxDepth = depth
	}
}

func WithDefaultPageSize(size int) Option {
	return func(s *CommentService) {
		s.defaultPageSize = size
	}
}

func WithMaxPageSize(size int) Option {
	return func(s *CommentService) {
		s.maxPageSize = size
	}
}
