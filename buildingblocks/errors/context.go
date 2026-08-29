package errors

import "context"

type internalErrorContextKey struct{}

// WithInternalError stores an error in the context for retrieval by middleware.
// Returns the original context if err is nil.
func WithInternalError(ctx context.Context, err error) context.Context {
	if err == nil {
		return ctx
	}
	return context.WithValue(ctx, internalErrorContextKey{}, err)
}

// InternalErrorFromContext retrieves an error stored by WithInternalError.
// Returns nil if no error is found in the context.
func InternalErrorFromContext(ctx context.Context) error {
	if val := ctx.Value(internalErrorContextKey{}); val != nil {
		if err, ok := val.(error); ok {
			return err
		}
	}
	return nil
}
