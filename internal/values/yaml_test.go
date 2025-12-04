package values

import "testing"

func TestParseYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		want    map[string]string
		wantErr bool
	}{
		{
			name: "Empty string returns empty map",
			yaml: "",
			want: map[string]string{},
		},
		{
			name: "Simple flat values",
			yaml: `
key1: value1
key2: value2
key3: value3
`,
			want: map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
		},
		{
			name: "Nested structure with dot notation",
			yaml: `
foo:
  bar:
    baz: value
`,
			want: map[string]string{
				"foo.bar.baz": "value",
			},
		},
		{
			name: "Multiple nested levels",
			yaml: `
app:
  database:
    host: localhost
    port: 5432
  cache:
    enabled: true
`,
			want: map[string]string{
				"app.database.host": "localhost",
				"app.database.port": "5432",
				"app.cache.enabled": "true",
			},
		},
		{
			name: "Array with bracket notation",
			yaml: `
items:
  - first
  - second
  - third
`,
			want: map[string]string{
				"items[0]": "first",
				"items[1]": "second",
				"items[2]": "third",
			},
		},
		{
			name: "Nested arrays and objects",
			yaml: `
servers:
  - name: server1
    port: 8080
  - name: server2
    port: 8081
`,
			want: map[string]string{
				"servers[0].name": "server1",
				"servers[0].port": "8080",
				"servers[1].name": "server2",
				"servers[1].port": "8081",
			},
		},
		{
			name: "Mixed types",
			yaml: `
string: hello
integer: 42
float: 3.14
boolean: true
null_value: null
`,
			want: map[string]string{
				"string":     "hello",
				"integer":    "42",
				"float":      "3.14",
				"boolean":    "true",
				"null_value": "",
			},
		},
		{
			name: "Empty string value",
			yaml: `
key: ""
`,
			want: map[string]string{
				"key": "",
			},
		},
		{
			name: "Numeric keys are preserved",
			yaml: `
"123": value1
"456": value2
`,
			want: map[string]string{
				"123": "value1",
				"456": "value2",
			},
		},
		{
			name: "Special characters in values",
			yaml: `
key: "value with spaces and special chars: !@#$%"
`,
			want: map[string]string{
				"key": "value with spaces and special chars: !@#$%",
			},
		},
		{
			name:    "Invalid YAML returns error",
			yaml:    "{ invalid yaml: [ no closing bracket",
			want:    nil,
			wantErr: true,
		},
		{
			name: "Deep nesting",
			yaml: `
a:
  b:
    c:
      d:
        e: deep_value
`,
			want: map[string]string{
				"a.b.c.d.e": "deep_value",
			},
		},
		{
			name: "Array of primitives",
			yaml: `
numbers:
  - 1
  - 2
  - 3
`,
			want: map[string]string{
				"numbers[0]": "1",
				"numbers[1]": "2",
				"numbers[2]": "3",
			},
		},
		{
			name: "Mixed nested structure",
			yaml: `
config:
  servers:
    - host: server1.com
      ports:
        - 80
        - 443
    - host: server2.com
      ports:
        - 8080
  enabled: true
`,
			want: map[string]string{
				"config.servers[0].host":     "server1.com",
				"config.servers[0].ports[0]": "80",
				"config.servers[0].ports[1]": "443",
				"config.servers[1].host":     "server2.com",
				"config.servers[1].ports[0]": "8080",
				"config.enabled":             "true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseYAML(tt.yaml)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if !mapsEqual(got, tt.want) {
				t.Errorf("parseYAML() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFlattenValue(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		value  interface{}
		want   map[string]string
	}{
		{
			name:   "String value",
			prefix: "key",
			value:  "value",
			want:   map[string]string{"key": "value"},
		},
		{
			name:   "Integer value",
			prefix: "key",
			value:  42,
			want:   map[string]string{"key": "42"},
		},
		{
			name:   "Boolean value",
			prefix: "key",
			value:  true,
			want:   map[string]string{"key": "true"},
		},
		{
			name:   "Float value",
			prefix: "key",
			value:  3.14,
			want:   map[string]string{"key": "3.14"},
		},
		{
			name:   "Nil value",
			prefix: "key",
			value:  nil,
			want:   map[string]string{"key": ""},
		},
		{
			name:   "Empty prefix with string",
			prefix: "",
			value:  "value",
			want:   map[string]string{"": "value"},
		},
		{
			name:   "Map value",
			prefix: "parent",
			value: map[string]interface{}{
				"child1": "value1",
				"child2": "value2",
			},
			want: map[string]string{
				"parent.child1": "value1",
				"parent.child2": "value2",
			},
		},
		{
			name:   "Array value",
			prefix: "list",
			value:  []interface{}{"a", "b", "c"},
			want: map[string]string{
				"list[0]": "a",
				"list[1]": "b",
				"list[2]": "c",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make(map[string]string)
			flattenValue(tt.prefix, tt.value, result)
			if !mapsEqual(result, tt.want) {
				t.Errorf("flattenValue() = %v, want %v", result, tt.want)
			}
		})
	}
}
