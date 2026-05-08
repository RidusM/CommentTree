package service

type Option func(*CommentService)

func MaxDepth(depth uint64) Option {
	return func(s *CommentService) {
		if depth > 0 {
			s.maxDepth = depth
		}
	}
}

func DefaultPageSize(size uint64) Option {
	return func(s *CommentService) {
		if size > 0 {
			s.pageSize = size
		}
	}
}

func MaxPageSize(size uint64) Option {
	return func(s *CommentService) {
		if size > 0 {
			s.maxPageSize = size
		}
	}
}
