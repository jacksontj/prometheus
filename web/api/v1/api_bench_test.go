package v1

import (
	"github.com/prometheus/prometheus/promql"
	"net/http/httptest"
	"testing"
)

// This is a global to avoid the benchmark being optimized away.
var testResponseWriter = httptest.ResponseRecorder{}

func BenchmarkRespond(b *testing.B) {
	b.ReportAllocs()
	points := []promql.Point{}
	for i := 0; i < 10000; i++ {
		points = append(points, promql.Point{V: float64(i * 1000000), T: int64(i)})
	}
	response := &queryData{
		ResultType: promql.ValueTypeMatrix,
		Result: promql.Matrix{
			promql.Series{
				Points: points,
				Metric: nil,
			},
		},
	}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		respond(&testResponseWriter, response)
	}
}
