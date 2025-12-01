package values

import "testing"

func TestEscapeValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "Simple alphanumeric value",
			value: "hello",
			want:  `"hello"`,
		},
		{
			name:  "Value with spaces",
			value: "hello world",
			want:  `"hello world"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeValue(tt.value)
			if got != tt.want {
				t.Errorf("escapeValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
