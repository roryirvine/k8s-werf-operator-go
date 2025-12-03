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
		{
			name:  "Empty value",
			value: "",
			want:  `""`,
		},
		{
			name:  "Value with double quotes",
			value: `say "hello"`,
			want:  `"say \"hello\""`,
		},
		{
			name:  "Value with single quotes",
			value: "it's working",
			want:  `"it's working"`,
		},
		{
			name:  "Value with equals signs",
			value: "key=value",
			want:  `"key=value"`,
		},
		{
			name:  "Value with newlines",
			value: "line1\nline2",
			want:  `"line1\nline2"`,
		},
		{
			name:  "Value with backslashes",
			value: `C:\path\to\file`,
			want:  `"C:\\path\\to\\file"`,
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
