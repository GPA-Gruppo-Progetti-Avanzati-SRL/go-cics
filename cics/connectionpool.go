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
	"github.com/rs/zerolog/log"
	"time"
	"unsafe"
)

var p *pool.ObjectPool
var ctx = context.Background()

func InitConnectionPool(config *ConnectionConfig) {
	tokenChannel := make(chan *C.CTG_ConnToken_t, 5)
	eciChannel := make(chan *C.ECI_ChannelToken_t)
	p = pool.NewObjectPool(ctx, &ConnectionFactory{Config: config, TokenChannel: tokenChannel, EciChannel: eciChannel}, &pool.ObjectPoolConfig{
		LIFO:                     true,
		MaxTotal:                 config.MaxTotal,
		MaxIdle:                  config.MaxIdle,
		MinIdle:                  config.MinIdle,
		BlockWhenExhausted:       true,
		MinEvictableIdleTime:     time.Duration(config.MaxIdleLifeTime) * time.Second,
		SoftMinEvictableIdleTime: 0,
		NumTestsPerEvictionRun:   0,
		EvictionPolicyName:       "",
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

	go ListeningClosure(tokenChannel)
	go ChannelClosure(eciChannel)

}

func ListeningClosure(tokenChannel chan *C.CTG_ConnToken_t) {
	for {
		val := <-tokenChannel
		log.Info().Msg("Ricevuto Token Procedo chiusura")
		closeGatewayConnection(val)
		log.Info().Msg("Effettuata chiusura")
		C.free(unsafe.Pointer(val))
	}
}
func ChannelClosure(channel chan *C.ECI_ChannelToken_t) {
	for {
		val := <-channel
		log.Info().Msg("Ricevuto Eci Token Procedo cancellazione canale")
		ctgRc := C.ECI_deleteChannel(val)

		if ctgRc != C.ECI_NO_ERROR {
			err := displayRc(ctgRc)
			log.Error().Err(err).Msgf("ECI_deleteChannel call failed : %v", err)
		}
		log.Info().Msg("Canale Eliminato")
	}
}

func CloseConnectionPool() {
	p.Close(ctx)
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
	return connection, nil
}

func ReturnConnection(cicsConnection *Connection) error {
	return p.ReturnObject(ctx, cicsConnection)
}
