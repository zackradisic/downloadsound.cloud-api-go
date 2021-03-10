package server

type contextKey int

const (
	// ContextBody is the context key to access body that has been validated through middleware
	ContextBody contextKey = iota
)
