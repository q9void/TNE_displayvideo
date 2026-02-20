package ptrutil

import (
	"testing"
)

func TestToPtr(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
	}{
		{"int", 42},
		{"string", "hello"},
		{"bool", true},
		{"float64", 3.14},
		{"zero int", 0},
		{"empty string", ""},
		{"false bool", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch v := tt.value.(type) {
			case int:
				ptr := ToPtr(v)
				if ptr == nil {
					t.Errorf("ToPtr() returned nil")
				}
				if *ptr != v {
					t.Errorf("ToPtr() = %v, want %v", *ptr, v)
				}
			case string:
				ptr := ToPtr(v)
				if ptr == nil {
					t.Errorf("ToPtr() returned nil")
				}
				if *ptr != v {
					t.Errorf("ToPtr() = %v, want %v", *ptr, v)
				}
			case bool:
				ptr := ToPtr(v)
				if ptr == nil {
					t.Errorf("ToPtr() returned nil")
				}
				if *ptr != v {
					t.Errorf("ToPtr() = %v, want %v", *ptr, v)
				}
			case float64:
				ptr := ToPtr(v)
				if ptr == nil {
					t.Errorf("ToPtr() returned nil")
				}
				if *ptr != v {
					t.Errorf("ToPtr() = %v, want %v", *ptr, v)
				}
			}
		})
	}
}

func TestToPtr_Inline(t *testing.T) {
	// Test inline usage (common OpenRTB pattern)
	ptr := ToPtr(1)
	if *ptr != 1 {
		t.Errorf("inline ToPtr(1) = %v, want 1", *ptr)
	}

	// Test that multiple calls create different pointers
	ptr1 := ToPtr(42)
	ptr2 := ToPtr(42)
	if ptr1 == ptr2 {
		t.Errorf("ToPtr() returned same pointer for different calls")
	}
}

func TestVal(t *testing.T) {
	tests := []struct {
		name string
		ptr  *int
		want int
	}{
		{"non-nil pointer", ToPtr(42), 42},
		{"nil pointer", nil, 0},
		{"zero value", ToPtr(0), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Val(tt.ptr)
			if got != tt.want {
				t.Errorf("Val() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVal_DifferentTypes(t *testing.T) {
	// Test string
	strPtr := ToPtr("hello")
	if Val(strPtr) != "hello" {
		t.Errorf("Val(string) = %v, want hello", Val(strPtr))
	}
	if Val((*string)(nil)) != "" {
		t.Errorf("Val(nil string) = %v, want empty string", Val((*string)(nil)))
	}

	// Test bool
	boolPtr := ToPtr(true)
	if Val(boolPtr) != true {
		t.Errorf("Val(bool) = %v, want true", Val(boolPtr))
	}
	if Val((*bool)(nil)) != false {
		t.Errorf("Val(nil bool) = %v, want false", Val((*bool)(nil)))
	}

	// Test float64
	floatPtr := ToPtr(3.14)
	if Val(floatPtr) != 3.14 {
		t.Errorf("Val(float64) = %v, want 3.14", Val(floatPtr))
	}
	if Val((*float64)(nil)) != 0.0 {
		t.Errorf("Val(nil float64) = %v, want 0.0", Val((*float64)(nil)))
	}
}

func TestValOr(t *testing.T) {
	tests := []struct {
		name       string
		ptr        *int
		defaultVal int
		want       int
	}{
		{"non-nil pointer", ToPtr(42), 100, 42},
		{"nil pointer with default", nil, 100, 100},
		{"zero value pointer", ToPtr(0), 100, 0},
		{"nil pointer with zero default", nil, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValOr(tt.ptr, tt.defaultVal)
			if got != tt.want {
				t.Errorf("ValOr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValOr_OpenRTBPattern(t *testing.T) {
	// Simulate OpenRTB timeout pattern
	type Request struct {
		TMax *int
	}

	// Request with timeout
	req1 := Request{TMax: ToPtr(500)}
	timeout1 := ValOr(req1.TMax, 1000)
	if timeout1 != 500 {
		t.Errorf("timeout with TMax set = %v, want 500", timeout1)
	}

	// Request without timeout (use default)
	req2 := Request{TMax: nil}
	timeout2 := ValOr(req2.TMax, 1000)
	if timeout2 != 1000 {
		t.Errorf("timeout with nil TMax = %v, want 1000", timeout2)
	}
}

func TestClone(t *testing.T) {
	tests := []struct {
		name string
		ptr  *int
		want *int
	}{
		{"non-nil pointer", ToPtr(42), ToPtr(42)},
		{"nil pointer", nil, nil},
		{"zero value", ToPtr(0), ToPtr(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Clone(tt.ptr)

			// Check nil cases
			if tt.ptr == nil {
				if got != nil {
					t.Errorf("Clone(nil) = %v, want nil", got)
				}
				return
			}

			// Check that clone has same value
			if *got != *tt.want {
				t.Errorf("Clone() value = %v, want %v", *got, *tt.want)
			}

			// Check that clone is a different pointer
			if got == tt.ptr {
				t.Errorf("Clone() returned same pointer instead of new pointer")
			}

			// Verify independence (modifying clone doesn't affect original)
			*got = 999
			if *tt.ptr == 999 {
				t.Errorf("modifying clone affected original")
			}
		})
	}
}

func TestClone_DifferentTypes(t *testing.T) {
	// Test string
	original := ToPtr("hello")
	clone := Clone(original)
	if *clone != *original {
		t.Errorf("Clone(string) value mismatch")
	}
	if clone == original {
		t.Errorf("Clone(string) returned same pointer")
	}

	// Test struct
	type Point struct {
		X, Y int
	}
	p := ToPtr(Point{X: 1, Y: 2})
	pClone := Clone(p)
	if pClone.X != 1 || pClone.Y != 2 {
		t.Errorf("Clone(struct) value mismatch")
	}
	if pClone == p {
		t.Errorf("Clone(struct) returned same pointer")
	}
}

func TestEqual(t *testing.T) {
	tests := []struct {
		name string
		a    *int
		b    *int
		want bool
	}{
		{"both nil", nil, nil, true},
		{"first nil", nil, ToPtr(42), false},
		{"second nil", ToPtr(42), nil, false},
		{"equal values", ToPtr(42), ToPtr(42), true},
		{"different values", ToPtr(42), ToPtr(99), false},
		{"both zero", ToPtr(0), ToPtr(0), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Equal(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("Equal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEqual_DifferentTypes(t *testing.T) {
	// Test string
	if !Equal(ToPtr("hello"), ToPtr("hello")) {
		t.Errorf("Equal(string) failed for equal strings")
	}
	if Equal(ToPtr("hello"), ToPtr("world")) {
		t.Errorf("Equal(string) returned true for different strings")
	}
	if !Equal((*string)(nil), (*string)(nil)) {
		t.Errorf("Equal(string) failed for both nil")
	}

	// Test bool
	if !Equal(ToPtr(true), ToPtr(true)) {
		t.Errorf("Equal(bool) failed for equal bools")
	}
	if Equal(ToPtr(true), ToPtr(false)) {
		t.Errorf("Equal(bool) returned true for different bools")
	}
}

// Benchmark tests to show performance
func BenchmarkToPtr(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = ToPtr(42)
	}
}

func BenchmarkVal(b *testing.B) {
	ptr := ToPtr(42)
	for i := 0; i < b.N; i++ {
		_ = Val(ptr)
	}
}

func BenchmarkValOr(b *testing.B) {
	ptr := ToPtr(42)
	for i := 0; i < b.N; i++ {
		_ = ValOr(ptr, 100)
	}
}

func BenchmarkClone(b *testing.B) {
	ptr := ToPtr(42)
	for i := 0; i < b.N; i++ {
		_ = Clone(ptr)
	}
}

func BenchmarkEqual(b *testing.B) {
	a := ToPtr(42)
	b1 := ToPtr(42)
	for i := 0; i < b.N; i++ {
		_ = Equal(a, b1)
	}
}

// Example usage demonstrating OpenRTB patterns
func ExampleToPtr() {
	// OpenRTB fields often use pointers for optional values
	// Instead of:
	// temp := 1
	// secure := &temp

	// Use:
	secure := ToPtr(1)
	_ = secure
}

func ExampleVal() {
	// Safely get value from potentially nil pointer
	var ptr *int = nil

	// Instead of:
	// var value int
	// if ptr != nil {
	//     value = *ptr
	// }

	// Use:
	value := Val(ptr) // Returns 0 if nil
	_ = value
}

func ExampleValOr() {
	// Get value with custom default
	type BidRequest struct {
		TMax *int
	}

	req := BidRequest{TMax: nil}

	// Instead of:
	// timeout := 1000
	// if req.TMax != nil {
	//     timeout = *req.TMax
	// }

	// Use:
	timeout := ValOr(req.TMax, 1000)
	_ = timeout
}

func ExampleClone() {
	// Create independent copy of pointer value
	original := ToPtr(42)
	copy := Clone(original)

	*copy = 99
	// original is still 42
	_ = original
}

func ExampleEqual() {
	// Compare two pointers
	a := ToPtr(42)
	b := ToPtr(42)
	c := (*int)(nil)

	_ = Equal(a, b) // true (same value)
	_ = Equal(a, c) // false (one is nil)
	_ = Equal(c, c) // true (both nil)
}
