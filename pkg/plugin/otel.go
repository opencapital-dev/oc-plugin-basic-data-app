package plugin

import (
	"github.com/grafana/grafana-plugin-sdk-go/backend/tracing"
	"go.opentelemetry.io/otel/trace"
)

const TracerName = "yfinance-ingestor-v2"

func Tracer() trace.Tracer {
	return tracing.DefaultTracer()
}
