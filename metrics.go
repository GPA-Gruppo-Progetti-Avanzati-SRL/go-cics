package cicsservice

import "go.opentelemetry.io/otel"
import "go.opentelemetry.io/otel/metric"

type Metrics struct {
	GetConnection       metric.Int64Counter
	ReturnedConnection  metric.Int64Counter
	ActiveConnection    metric.Int64UpDownCounter
	CreateConnection    metric.Int64Counter
	DestroyConnection   metric.Int64Counter
	TransactionDuration metric.Int64Histogram
}

var meter = otel.Meter("cics-service")

func NewMetrics() *Metrics {
	gc, _ := meter.Int64Counter(
		"cics_get_connection",
		metric.WithDescription("Number of get connections requested."),
	)

	rc, _ := meter.Int64Counter(
		"cics_returned_connection",
		metric.WithDescription("Number of get connections returned to the pool."),
	)
	ac, _ := meter.Int64UpDownCounter(
		"cics_active_connection",
		metric.WithDescription("Number of active connections created"),
	)
	cc, _ := meter.Int64Counter(

		"cics_create_connection",
		metric.WithDescription("Number of connections created"),
	)
	dc, _ := meter.Int64Counter(
		"cics_destroy_connection",
		metric.WithDescription("Number of  connections destroyed "),
	)
	td, _ := meter.Int64Histogram("cics_transaction_duration", metric.WithDescription("The duration of a transaction."), metric.WithUnit("ms"))

	m := &Metrics{
		GetConnection:       gc,
		ReturnedConnection:  rc,
		ActiveConnection:    ac,
		CreateConnection:    cc,
		DestroyConnection:   dc,
		TransactionDuration: td,
	}

	return m
}
