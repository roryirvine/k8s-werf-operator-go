package values

import "testing"

func TestMergeMaps(t *testing.T) {
	tests := []struct {
		name string
		maps []map[string]string
		want map[string]string
	}{
		{
			name: "Empty input returns empty map",
			maps: []map[string]string{},
			want: map[string]string{},
		},
		{
			name: "Single map is returned as-is",
			maps: []map[string]string{
				{"key1": "value1", "key2": "value2"},
			},
			want: map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name: "Two maps with no conflicts",
			maps: []map[string]string{
				{"key1": "value1"},
				{"key2": "value2"},
			},
			want: map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name: "Two maps with conflict - later wins",
			maps: []map[string]string{
				{"key1": "old-value"},
				{"key1": "new-value"},
			},
			want: map[string]string{"key1": "new-value"},
		},
		{
			name: "Multiple maps with mixed conflicts",
			maps: []map[string]string{
				{"key1": "value1", "key2": "value2"},
				{"key2": "override2", "key3": "value3"},
				{"key1": "override1", "key4": "value4"},
			},
			want: map[string]string{
				"key1": "override1",
				"key2": "override2",
				"key3": "value3",
				"key4": "value4",
			},
		},
		{
			name: "Empty map in sequence doesn't affect result",
			maps: []map[string]string{
				{"key1": "value1"},
				{},
				{"key2": "value2"},
			},
			want: map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name: "All empty maps",
			maps: []map[string]string{
				{},
				{},
				{},
			},
			want: map[string]string{},
		},
		{
			name: "Nil maps are handled gracefully",
			maps: []map[string]string{
				{"key1": "value1"},
				nil,
				{"key2": "value2"},
			},
			want: map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name: "Later map can override with empty string",
			maps: []map[string]string{
				{"key1": "value1"},
				{"key1": ""},
			},
			want: map[string]string{"key1": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeMaps(tt.maps...)
			if !mapsEqual(got, tt.want) {
				t.Errorf("mergeMaps() = %v, want %v", got, tt.want)
			}
		})
	}
}
