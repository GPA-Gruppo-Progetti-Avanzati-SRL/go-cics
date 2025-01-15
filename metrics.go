package cicsservice

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	GetConnection       prometheus.Counter
	ReturnedConnection  prometheus.Counter
	ActiveConnection    prometheus.Gauge
	CreateConnection    prometheus.Counter
	DestroyConnection   prometheus.Counter
	TransactionDuration *prometheus.HistogramVec
}

var metrics *Metrics

func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		GetConnection: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "cics_get_connection_count",
			Help: "Number of get connections requested.",
		}),
		ReturnedConnection: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "cics_returned_connection_count",
			Help: "Number of get connections returned to the pool.",
		}),
		ActiveConnection: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "cics_active_connection_count",
			Help: "Number of active connections created",
		}),
		CreateConnection: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "cics_create_connection_count",
			Help: "Number of connections created",
		}),
		DestroyConnection: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "cics_destroy_connection_count",
			Help: "Number of  connections destroyed ",
		}),
		/*TransactionDuration: prometheus.NewHistogramVec(prometheus.HistogramVecOpts{
			HistogramOpts: prometheus.HistogramOpts{

			},
			VariableLabels: []string{"routine"},
		}),*/
		TransactionDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "cics_transaction_routine_duration",
			Help:    "Duration of transactions",
			Buckets: prometheus.LinearBuckets(0.01, 0.01, 10),
		}, []string{"routine"}),
	}
	reg.MustRegister(m.GetConnection)
	reg.MustRegister(m.ReturnedConnection)
	reg.MustRegister(m.ActiveConnection)
	reg.MustRegister(m.CreateConnection)
	reg.MustRegister(m.DestroyConnection)
	reg.MustRegister(m.TransactionDuration)
	return m
}
