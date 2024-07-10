package cics

import (
	"context"
	pool "github.com/jolestar/go-commons-pool/v2"
	"github.com/rs/zerolog/log"
	"sync"
	"time"
)

var p *pool.ObjectPool
var ctx = context.Background()

type PortPool struct {
	ports chan int
	mu    sync.Mutex
}

func NewPortPool(start, end int) *PortPool {
	pool := &PortPool{
		ports: make(chan int, end-start+1),
	}
	for i := start; i <= end; i++ {
		pool.ports <- i
	}
	return pool
}

// Funzione per richiedere una porta
func (p *PortPool) GetPort() int {
	return <-p.ports
}

// Funzione per liberare una porta
func (p *PortPool) ReleasePort(port int) {
	p.ports <- port
}

func InitConnectionPool(config *ConnectionConfig) {

	connFactory := &ConnectionFactory{Config: config}
	if config.UseProxy {
		if config.ProxyPort == 0 {
			log.Debug().Msg("ProxyPort no setted setting default 18080")
			config.ProxyPort = 18080
		}
		proxyPortStart := config.ProxyPort
		proxyPortEnd := proxyPortStart + config.MaxTotal
		connFactory.PortPool = NewPortPool(proxyPortStart, proxyPortEnd)
	}

	p = pool.NewObjectPool(ctx, connFactory, &pool.ObjectPoolConfig{
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
