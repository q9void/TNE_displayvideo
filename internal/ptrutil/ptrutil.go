// Package ptrutil provides pointer utility functions
package ptrutil

// ToPtr returns a pointer to the given value
// Useful for inline pointer creation without temporary variables
//
// Example:
//
//	secure := ptrutil.ToPtr(1)
//	// Instead of: temp := 1; secure := &temp
func ToPtr[T any](v T) *T {
	return &v
}

// Val returns the value that ptr points to, or the zero value if ptr is nil
// Useful for safely dereferencing pointers
//
// Example:
//
//	value := ptrutil.Val(secure)
//	// Instead of: if secure != nil { value = *secure }
func Val[T any](ptr *T) T {
	if ptr == nil {
		var zero T
		return zero
	}
	return *ptr
}

// ValOr returns the value that ptr points to, or defaultVal if ptr is nil
// Useful when you need a specific default instead of the zero value
//
// Example:
//
//	timeout := ptrutil.ValOr(req.TMax, 1000)
func ValOr[T any](ptr *T, defaultVal T) T {
	if ptr == nil {
		return defaultVal
	}
	return *ptr
}

// Clone returns a new pointer to a copy of the value
// Useful for copying pointer values
//
// Example:
//
//	copy := ptrutil.Clone(original)
func Clone[T any](ptr *T) *T {
	if ptr == nil {
		return nil
	}
	v := *ptr
	return &v
}

// Equal compares two pointers for equality
// Returns true if both are nil or both point to equal values
//
// Example:
//
//	if ptrutil.Equal(a, b) { ... }
func Equal[T comparable](a, b *T) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
