package mysql

import (
	"reflect"
	"testing"
)

func TestVisibleDatabases(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "user databases hide the system ones",
			in:   []string{"book", "information_schema", "mysql", "performance_schema", "sys"},
			want: []string{"book"},
		},
		{
			// A freshly installed server must not look like a broken connection.
			name: "fresh server keeps the system schemas",
			in:   []string{"information_schema", "mysql", "performance_schema", "sys"},
			want: []string{"information_schema", "mysql", "performance_schema", "sys"},
		},
		{
			name: "no databases at all stays empty",
			in:   []string{},
			want: []string{},
		},
		{
			name: "case insensitive filter",
			in:   []string{"MySQL", "SYS"},
			want: []string{"MySQL", "SYS"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := visibleDatabases(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("visibleDatabases(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
