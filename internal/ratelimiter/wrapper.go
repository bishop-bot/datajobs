package ratelimiter

import (
	"context"
	"reflect"
)

// Wrapper wraps an interface with rate limiting for all its methods.
type Wrapper struct {
	bucket *TokenBucket
	target interface{}
}

// NewWrapper creates a new rate-limited wrapper around the target interface.
func NewWrapper(target interface{}, requestsPerMin int) (*Wrapper, error) {
	return &Wrapper{
		bucket: NewTokenBucket(requestsPerMin),
		target: target,
	}, nil
}

// NewWrapperWithBucket creates a wrapper with a pre-configured token bucket.
func NewWrapperWithBucket(target interface{}, bucket *TokenBucket) *Wrapper {
	return &Wrapper{
		bucket: bucket,
		target: target,
	}
}

// Bucket returns the underlying token bucket.
func (w *Wrapper) Bucket() *TokenBucket {
	return w.bucket
}

// Target returns the wrapped target.
func (w *Wrapper) Target() interface{} {
	return w.target
}

// Call wraps a method call with rate limiting.
// The method must have context as its first parameter.
func (w *Wrapper) Call(ctx context.Context, methodName string, args ...interface{}) ([]interface{}, error) {
	// First, apply rate limiting
	if err := w.bucket.Allow(ctx); err != nil {
		return nil, err
	}

	// Get method from target
	targetVal := reflect.ValueOf(w.target)
	method := targetVal.MethodByName(methodName)
	if !method.IsValid() {
		return nil, &MethodNotFoundError{Method: methodName}
	}

	// Build arguments: context + provided args
	argCount := len(args) + 1 // +1 for context
	argVals := make([]reflect.Value, argCount)
	argVals[0] = reflect.ValueOf(ctx)
	for i, arg := range args {
		argVals[i+1] = reflect.ValueOf(arg)
	}

	// Call the method
	results := method.Call(argVals)

	// Convert results
	output := make([]interface{}, len(results))
	for i, v := range results {
		if v.CanInterface() {
			output[i] = v.Interface()
		}
	}

	return output, nil
}

// MethodNotFoundError is returned when a method is not found on the wrapped target.
type MethodNotFoundError struct {
	Method string
}

func (e *MethodNotFoundError) Error() string {
	return "method not found: " + e.Method
}
