package rewriter

import (
	"strings"
	"testing"
)

func TestRewriteMapStringString(t *testing.T) {
	tests := []struct {
		name     string
		mapJSON  string
		expected string
	}{
		{
			name:     "map with labels",
			mapJSON:  `{"metadata":{"labels":{"hack.test.some-value-1":"true","hack.test.some-value-2":"true","hack.test.some-value-3":"true"}}}`,
			expected: `{"metadata":{"labels":{"renamed.some-value-1":"true","renamed.some-value-2":"true","renamed.some-value-3":"true"}}}`,
		},
		{
			name:     "map with nulls",
			mapJSON:  `{"metadata":{"labels":{"hack.test.some-value-1":null,"hack.test.some-value-5":null,"hack.test.some-value-6":"true","hack.test.some-value-7":"true","hack.test.some-value-8":"true"}}}`,
			expected: `{"metadata":{"labels":{"renamed.some-value-1":null,"renamed.some-value-5":null,"renamed.some-value-6":"true","renamed.some-value-7":"true","renamed.some-value-8":"true"}}}`,
		},
	}

	trFn := func(k, v string) (string, string) {
		return strings.ReplaceAll(k, "hack.test", "renamed"), v
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := RewriteMapStringString([]byte(test.mapJSON), "metadata.labels", trFn)

			if err != nil {
				t.Fatalf("RewriteMapStringString() unexpected error = %v", err)
			}

			if string(actual) != test.expected {
				t.Fatalf("RewriteMapStringString()\nactual = %s,\nexpected = %s", string(actual), test.expected)
			}
		})
	}
}
