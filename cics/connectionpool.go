package cics

import (
	"context"
	pool "github.com/jolestar/go-commons-pool/v2"
	"github.com/rs/zerolog/log"
	"time"
)

var p *pool.ObjectPool
var ctx = context.Background()

func InitConnectionPool(config *ConnectionConfig) {

	p = pool.NewObjectPool(ctx, &ConnectionFactory{Config: config}, &pool.ObjectPoolConfig{
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

}

func CloseConnectionPool() {

	p.Close(ctx)
}

func GetConnection() (*Connection, error) {
	cicsConnection, err := p.BorrowObject(ctx)
	if err != nil {
		return nil, err
	}
	connection := cicsConnection.(*Connection)
	return connection, nil
}

func ReturnConnection(cicsConnection *Connection) error {
	return p.ReturnObject(ctx, cicsConnection)
}
