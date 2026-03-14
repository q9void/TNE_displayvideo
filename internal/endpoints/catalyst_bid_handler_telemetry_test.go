package endpoints

import (
	"encoding/json"
	"testing"
)

func TestCountExtEIDs(t *testing.T) {
	tests := []struct {
		name     string
		ext      json.RawMessage
		expected int
	}{
		{
			name:     "nil ext",
			ext:      nil,
			expected: 0,
		},
		{
			name:     "empty ext",
			ext:      json.RawMessage(`{}`),
			expected: 0,
		},
		{
			name:     "two eids in ext",
			ext:      json.RawMessage(`{"eids":[{"source":"a.com"},{"source":"b.com"}]}`),
			expected: 2,
		},
		{
			name:     "ext without eids key",
			ext:      json.RawMessage(`{"consent":"abc"}`),
			expected: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := countExtEIDs(tc.ext)
			if got != tc.expected {
				t.Errorf("expected %d, got %d", tc.expected, got)
			}
		})
	}
}
