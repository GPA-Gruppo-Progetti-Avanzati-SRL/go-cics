package cicsservice

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
	"github.com/rs/zerolog/log"
	"go.uber.org/fx"
	"time"
	"unsafe"
)

type EvictionPolicy struct {
}

// Evict do evict by config
func (p *EvictionPolicy) Evict(config *pool.EvictionConfig, underTest *pool.PooledObject, idleCount int) bool {
	idleTime := underTest.GetIdleTime()
	log.Trace().Msgf("Test Evict Config idle soft evict time : %d - idleTime : %d - idleCount : %d   - config.Minidle : %d - config.IdleEvictTime : %d  ", config.IdleSoftEvictTime, idleTime, idleCount, config.MinIdle, config.IdleEvictTime)

	if (config.IdleSoftEvictTime < idleTime &&
		config.MinIdle < idleCount) ||
		config.IdleEvictTime < idleTime {
		log.Trace().Msg("Test Evict True")
		return true
	}
	log.Trace().Msg("Test Evict False")
	return false
}

type Service struct {
	p        *pool.ObjectPool
	Routines map[string]*RoutineConfig
	Metrics  *Metrics
}

func NewService(lc fx.Lifecycle, config *ConnectionConfig, metrics *Metrics, routineConfig []*RoutineConfig) *Service {
	cp := &Service{
		Routines: GetRoutines(routineConfig),
		Metrics:  metrics,
	}

	TokenChannel = make(chan *C.CTG_ConnToken_t, config.MaxTotal)
	EciChannel = make(chan *C.ECI_ChannelToken_t)
	pool.RegistryEvictionPolicy("CicsEvictionPolicy", &EvictionPolicy{})

	pc := &pool.ObjectPoolConfig{
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
		TimeBetweenEvictionRuns:  60 * time.Second,
		EvictionContext:          nil,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			cp.p = pool.NewObjectPool(ctx, &ConnectionFactory{Config: config, Metrics: metrics}, pc)
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
		},
		OnStop: func(ctx context.Context) error {

			log.Info().Msg("Closing connection pool")
			if cp.p != nil {
				cp.p.Close(ctx)
			}

			close(EciChannel)
			close(TokenChannel)
			return nil
		},
	})

	return cp
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

func (s *Service) GetConnection(ctx context.Context) (*Connection, error) {
	cicsConnection, err := s.p.BorrowObject(ctx)
	if err != nil {
		log.Error().Msgf("Error getting connection from pool: %v", err)
		return nil, err
	}

	connection := cicsConnection.(*Connection)
	if connection.ConnectionToken == nil {
		log.Warn().Msg("ConnectionToken is nil invalidate")
		s.p.InvalidateObject(ctx, cicsConnection)
		return s.GetConnection(ctx)
	}
	s.Metrics.GetConnection.Add(ctx, 1)
	s.Metrics.ActiveConnection.Add(ctx, 1)
	return connection, nil
}

func (s *Service) ReturnConnection(ctx context.Context, cicsConnection *Connection) error {
	s.Metrics.ReturnedConnection.Add(ctx, 1)
	s.Metrics.ActiveConnection.Add(ctx, -1)
	return s.p.ReturnObject(ctx, cicsConnection)
}

func GetRoutines(routinescfg []*RoutineConfig) map[string]*RoutineConfig {
	routines := make(map[string]*RoutineConfig)
	for _, v := range routinescfg {

		routines[v.Name] = v
	}

	return routines
}
