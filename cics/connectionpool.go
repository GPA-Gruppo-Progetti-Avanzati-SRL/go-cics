package cics

/*
#include <ctgclient_eci.h>
#include <string.h>
#include <stdlib.h>
#include <stdio.h>
#include <fcntl.h>
*/
import "C"
import (
	"context"
	pool "github.com/jolestar/go-commons-pool/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	"time"
	"unsafe"
)

var p *pool.ObjectPool
var ctx = context.Background()

type Metrics struct {
	GetConnection      prometheus.Counter
	ReturnedConnection prometheus.Counter
	ActiveConnection   prometheus.Gauge
}

var metrics *Metrics

func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		GetConnection: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "get_connection_count",
			Help: "Number of get connections requested.",
		}),
		ReturnedConnection: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "returned_connection_count",
			Help: "Number of get connections returned to the pool.",
		}),
		ActiveConnection: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "active_connection_count",
			Help: "Number of active connections.",
		}),
	}
	reg.MustRegister(m.GetConnection)
	reg.MustRegister(m.ReturnedConnection)
	reg.MustRegister(m.ActiveConnection)
	return m
}

type CicsEvictionPolicy struct {
}

// Evict do evict by config
func (p *CicsEvictionPolicy) Evict(config *pool.EvictionConfig, underTest *pool.PooledObject, idleCount int) bool {
	idleTime := underTest.GetIdleTime()
	log.Trace().Msgf("Test Evict Config idle soft evict time : %d - idleTime : %d - idleCount : %d   - config.Minidle : %d - config.IdleEvictTime : %d  ",,config.IdleSoftEvictTime,idleTime,idleCount,config.MinIdle,config.IdleEvictTime )

	if (config.IdleSoftEvictTime < idleTime &&
		config.MinIdle < idleCount) ||
		config.IdleEvictTime < idleTime {
		log.Trace().Msg("Test Evict True")
		return true
	}
	log.Trace().Msg("Test Evict False")
	return false
}

func InitConnectionPool(config *ConnectionConfig, reg *prometheus.Registerer) error {
	TokenChannel = make(chan *C.CTG_ConnToken_t, config.MaxTotal)
	EciChannel = make(chan *C.ECI_ChannelToken_t)
	pool.RegistryEvictionPolicy("CicsEvictionPolicy", &CicsEvictionPolicy{})
	metrics = NewMetrics(*reg)
	p = pool.NewObjectPool(ctx, &ConnectionFactory{Config: config}, &pool.ObjectPoolConfig{
		LIFO:                     true,
		MaxTotal:                 config.MaxTotal,
		MaxIdle:                  config.MaxIdle,
		MinIdle:                  config.MinIdle,
		TestOnBorrow:             true,
		TestOnReturn:             true,
		BlockWhenExhausted:       true,
		MinEvictableIdleTime:     time.Duration(config.MaxIdleLifeTime) * time.Second,
		SoftMinEvictableIdleTime: 1,
		NumTestsPerEvictionRun:   1,
		EvictionPolicyName:       "CicsEvictionPolicy",
		TestWhileIdle:            true,
		TimeBetweenEvictionRuns:  time.Duration(config.MaxIdleLifeTime/2) * time.Second,
		EvictionContext:          nil,
	})
	if config.UseProxy {
		proxyready := make(chan bool)
		if config.ProxyPort == 0 {
			log.Debug().Msg("ProxyPort no setted setting default 18080")
			config.ProxyPort = 18080
		}
		go Encrypt(config, proxyready)
		log.Info().Msg("Wait opening socket")
		<-proxyready
		log.Info().Msg("Socket opened")
	}

	go ListeningClosure()
	go ChannelClosure()
	return nil
}

func ListeningClosure() {
	for {
		val := <-TokenChannel
		log.Trace().Msg("Ricevuto Token Procedo chiusura")
		ctgRc := C.CTG_closeGatewayConnection(val)
		if ctgRc != C.CTG_OK {
			err := displayRc(ctgRc)
			log.Error().Err(err).Msgf("Failed to close connection to CICS Transaction Gateway %v", err)

		} else {
			log.Trace().Msg("Closed connection to CICS Transaction Gateway")
		}
		C.free(unsafe.Pointer(val))
	}
}
func ChannelClosure() {
	for {
		val := <-EciChannel
		log.Trace().Msg("Ricevuto Eci Token Procedo cancellazione canale")
		ctgRc := C.ECI_deleteChannel(val)
		if ctgRc != C.ECI_NO_ERROR {
			err := displayRc(ctgRc)
			log.Error().Err(err).Msgf("ECI_deleteChannel call failed : %v", err)
		} else {
			log.Trace().Msg("Canale Eliminato")
		}

	}
}

func CloseConnectionPool() {
	log.Info().Msg("Closing connection pool")
	p.Close(ctx)
	close(EciChannel)
	close(TokenChannel)
}

func GetConnection() (*Connection, error) {
	cicsConnection, err := p.BorrowObject(ctx)
	if err != nil {
		log.Error().Msgf("Error getting connection from pool: %v", err)
		return nil, err
	}

	connection := cicsConnection.(*Connection)
	if connection.ConnectionToken == nil {
		log.Debug().Msg("ConnectionToken is nil invalidate")
		p.InvalidateObject(ctx, cicsConnection)
		return GetConnection()
	}
	metrics.GetConnection.Inc()
	metrics.ActiveConnection.Inc()
	return connection, nil
}

func ReturnConnection(cicsConnection *Connection) error {
	metrics.ReturnedConnection.Inc()
	metrics.ActiveConnection.Dec()
	return p.ReturnObject(ctx, cicsConnection)
}
