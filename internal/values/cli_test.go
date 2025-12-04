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
			want:  "hello",
		},
		{
			name:  "Value with spaces",
			value: "hello world",
			want:  "hello world",
		},
		{
			name:  "Empty value",
			value: "",
			want:  "",
		},
		{
			name:  "Value with commas (Helm separator)",
			value: "value,with,commas",
			want:  `value\,with\,commas`,
		},
		{
			name:  "Value with equals signs (Helm key=value separator)",
			value: "key=value",
			want:  `key\=value`,
		},
		{
			name:  "Value with backslashes",
			value: `C:\path\to\file`,
			want:  `C:\\path\\to\\file`,
		},
		{
			name:  "Value with brackets (Helm array notation)",
			value: "servers[0]",
			want:  `servers\[0\]`,
		},
		{
			name:  "Value with multiple special characters",
			value: "path=value,with[brackets]",
			want:  `path\=value\,with\[brackets\]`,
		},
		{
			name:  "Backslash before other special chars",
			value: `\,\=\[\]`,
			want:  `\\\,\\\=\\\[\\\]`,
		},
		{
			name:  "Database connection string",
			value: "postgres://user:pass@localhost:5432/db",
			want:  `postgres://user:pass@localhost:5432/db`,
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
