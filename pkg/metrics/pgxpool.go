package metrics

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

type PGXPoolCollector struct {
	pool *pgxpool.Pool

	maxConns               *prometheus.Desc // gauge
	totalConns             *prometheus.Desc // gauge
	idleConns              *prometheus.Desc // gauge
	acquiredConns          *prometheus.Desc // gauge
	constructingConns      *prometheus.Desc // gauge
	acquireCount           *prometheus.Desc // counter
	acquireDurationSeconds *prometheus.Desc // counter (сумма времени)
	canceledAcquireCount   *prometheus.Desc // counter
	emptyAcquireCount      *prometheus.Desc // counter
	emptyAcquireWaitSec    *prometheus.Desc // counter (сумма времени ожидания)
	newConnsCount          *prometheus.Desc // counter
	maxIdleDestroyCount    *prometheus.Desc // counter
	maxLifeDestroyCount    *prometheus.Desc // counter
}

func NewPGXPoolCollector(pool *pgxpool.Pool, namespace, service string) *PGXPoolCollector {
	ns := func(name string) string { return prometheus.BuildFQName(namespace, service, name) }

	return &PGXPoolCollector{
		pool: pool,

		maxConns:               prometheus.NewDesc(ns("pgx_max_conns"), "Max size of the pgx pool.", nil, nil),
		totalConns:             prometheus.NewDesc(ns("pgx_total_conns"), "Total number of connections (constructing + idle + acquired).", nil, nil),
		idleConns:              prometheus.NewDesc(ns("pgx_idle_conns"), "Current number of idle connections.", nil, nil),
		acquiredConns:          prometheus.NewDesc(ns("pgx_acquired_conns"), "Current number of acquired (checked-out) connections.", nil, nil),
		constructingConns:      prometheus.NewDesc(ns("pgx_constructing_conns"), "Current number of connections in construction.", nil, nil),
		acquireCount:           prometheus.NewDesc(ns("pgx_acquire_total"), "Cumulative successful acquires from the pool.", nil, nil),
		acquireDurationSeconds: prometheus.NewDesc(ns("pgx_acquire_duration_seconds_total"), "Total time spent acquiring connections (seconds).", nil, nil),
		canceledAcquireCount:   prometheus.NewDesc(ns("pgx_acquire_canceled_total"), "Cumulative acquires canceled by context.", nil, nil),
		emptyAcquireCount:      prometheus.NewDesc(ns("pgx_acquire_empty_total"), "Cumulative successful acquires that had to wait because pool was empty.", nil, nil),
		emptyAcquireWaitSec:    prometheus.NewDesc(ns("pgx_acquire_empty_wait_seconds_total"), "Total time spent waiting for a connection when pool was empty (seconds).", nil, nil),
		newConnsCount:          prometheus.NewDesc(ns("pgx_new_conns_total"), "Cumulative number of new connections opened.", nil, nil),
		maxIdleDestroyCount:    prometheus.NewDesc(ns("pgx_max_idle_destroy_total"), "Cumulative number of connections destroyed due to MaxConnIdleTime.", nil, nil),
		maxLifeDestroyCount:    prometheus.NewDesc(ns("pgx_max_lifetime_destroy_total"), "Cumulative number of connections destroyed due to MaxConnLifetime.", nil, nil),
	}
}

func (c *PGXPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.maxConns
	ch <- c.totalConns
	ch <- c.idleConns
	ch <- c.acquiredConns
	ch <- c.constructingConns
	ch <- c.acquireCount
	ch <- c.acquireDurationSeconds
	ch <- c.canceledAcquireCount
	ch <- c.emptyAcquireCount
	ch <- c.emptyAcquireWaitSec
	ch <- c.newConnsCount
	ch <- c.maxIdleDestroyCount
	ch <- c.maxLifeDestroyCount
}

func (c *PGXPoolCollector) Collect(ch chan<- prometheus.Metric) {
	if c.pool == nil {
		return
	}
	s := c.pool.Stat()

	ch <- prometheus.MustNewConstMetric(c.maxConns, prometheus.GaugeValue, float64(s.MaxConns()))
	ch <- prometheus.MustNewConstMetric(c.totalConns, prometheus.GaugeValue, float64(s.TotalConns()))
	ch <- prometheus.MustNewConstMetric(c.idleConns, prometheus.GaugeValue, float64(s.IdleConns()))
	ch <- prometheus.MustNewConstMetric(c.acquiredConns, prometheus.GaugeValue, float64(s.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(c.constructingConns, prometheus.GaugeValue, float64(s.ConstructingConns()))

	ch <- prometheus.MustNewConstMetric(c.acquireCount, prometheus.CounterValue, float64(s.AcquireCount()))
	ch <- prometheus.MustNewConstMetric(c.acquireDurationSeconds, prometheus.CounterValue, secs(s.AcquireDuration()))
	ch <- prometheus.MustNewConstMetric(c.canceledAcquireCount, prometheus.CounterValue, float64(s.CanceledAcquireCount()))
	ch <- prometheus.MustNewConstMetric(c.emptyAcquireCount, prometheus.CounterValue, float64(s.EmptyAcquireCount()))
	ch <- prometheus.MustNewConstMetric(c.emptyAcquireWaitSec, prometheus.CounterValue, secs(s.EmptyAcquireWaitTime()))
	ch <- prometheus.MustNewConstMetric(c.newConnsCount, prometheus.CounterValue, float64(s.NewConnsCount()))
	ch <- prometheus.MustNewConstMetric(c.maxIdleDestroyCount, prometheus.CounterValue, float64(s.MaxIdleDestroyCount()))
	ch <- prometheus.MustNewConstMetric(c.maxLifeDestroyCount, prometheus.CounterValue, float64(s.MaxLifetimeDestroyCount()))
}

func secs(d time.Duration) float64 { return float64(d) / float64(time.Second) }
