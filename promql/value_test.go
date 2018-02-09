package promql

import (
	"encoding/json"
	"testing"
)

func TestPointMarshal(t *testing.T) {
	tests := []struct {
		point Point
		val   string
	}{
		{
			point: Point{100, 500},
			val:   `[0.1,"500"]`,
		},
		{
			point: Point{10, 5.2},
			val:   `[0.01,"5.2"]`,
		},
		{
			point: Point{1000, 5},
			val:   `[1,"5"]`,
		},
	}

	for _, test := range tests {
		b, _ := json.Marshal(test.point)
		if string(b) != test.val {
			t.Fatalf("mismatch expected=%s actual=%s", test.val, string(b))
		}
	}

}
