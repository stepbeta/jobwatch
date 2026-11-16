package diff

import (
	"reflect"
	"strings"
	"testing"
)

func TestLines(t *testing.T) {
	tests := []struct {
		name string
		old  string
		new  string
		want []Line
	}{
		{
			name: "identical",
			old:  "a\nb",
			new:  "a\nb",
			want: []Line{{OpEqual, "a"}, {OpEqual, "b"}},
		},
		{
			name: "added line",
			old:  "a\nc",
			new:  "a\nb\nc",
			want: []Line{{OpEqual, "a"}, {OpAdded, "b"}, {OpEqual, "c"}},
		},
		{
			name: "removed line",
			old:  "a\nb\nc",
			new:  "a\nc",
			want: []Line{{OpEqual, "a"}, {OpRemoved, "b"}, {OpEqual, "c"}},
		},
		{
			name: "changed line",
			old:  "a\nold\nc",
			new:  "a\nnew\nc",
			want: []Line{{OpEqual, "a"}, {OpRemoved, "old"}, {OpAdded, "new"}, {OpEqual, "c"}},
		},
		{
			name: "empty old",
			old:  "",
			new:  "a\nb",
			want: []Line{{OpAdded, "a"}, {OpAdded, "b"}},
		},
		{
			name: "empty new",
			old:  "a\nb",
			new:  "",
			want: []Line{{OpRemoved, "a"}, {OpRemoved, "b"}},
		},
		{
			name: "crlf and trailing spaces",
			old:  "a  \r\nb\r\n",
			new:  "a\r\nb\r\nc\r\n",
			want: []Line{{OpEqual, "a"}, {OpEqual, "b"}, {OpAdded, "c"}},
		},
		{
			name: "blank line added",
			old:  "a\nb",
			new:  "a\n\nb",
			want: []Line{{OpEqual, "a"}, {OpAdded, ""}, {OpEqual, "b"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Lines(tt.old, tt.new)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Lines(%q, %q) = %#v, want %#v", tt.old, tt.new, got, tt.want)
			}
		})
	}
}

func TestLinesPreservesAddedAndRemovedText(t *testing.T) {
	old := strings.Join([]string{"one", "two", "three"}, "\n")
	new := strings.Join([]string{"one", "TWO", "three", "four"}, "\n")
	got := Lines(old, new)
	var added, removed []string
	for _, line := range got {
		switch line.Op {
		case OpAdded:
			added = append(added, line.Text)
		case OpRemoved:
			removed = append(removed, line.Text)
		}
	}
	if strings.Join(removed, "|") != "two" {
		t.Errorf("removed = %v", removed)
	}
	if strings.Join(added, "|") != "TWO|four" {
		t.Errorf("added = %v", added)
	}
}

func TestLinesCoarseFallbackKeepsOrder(t *testing.T) {
	old := strings.Repeat("old line\n", 2001)
	new := strings.Repeat("new line\n", 2001)
	got := Lines(old, new)
	ops := map[Op]int{}
	for _, line := range got {
		ops[line.Op]++
	}
	if ops[OpRemoved] != 2001 || ops[OpAdded] != 2001 || ops[OpEqual] != 0 {
		t.Errorf("ops = %v", ops)
	}
}
