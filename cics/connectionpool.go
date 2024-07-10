package cics

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	pool "github.com/jolestar/go-commons-pool/v2"
	"github.com/rs/zerolog/log"
	"os"
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

		log.Info().Msgf("Reading %v as certificate, %v as key and %v as root certificate", config.SSLClientCertificate, config.SSLClientKey, config.SSLRootCaCertificate)
		cert, err := tls.LoadX509KeyPair(config.SSLClientCertificate, config.SSLClientKey)
		caCert, err := os.ReadFile(config.SSLRootCaCertificate)
		if err != nil {
			log.Fatal().Err(err).Msgf("failed to load cert: %s", err.Error())
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		tlsConfig := &tls.Config{
			InsecureSkipVerify: config.InsecureSkipVerify,
			Certificates:       []tls.Certificate{cert}, // this certificate is used to sign the handshake
			RootCAs:            caCertPool,              // this is used to validate the server certificate
		}
		if config.ProxyPort == 0 {
			log.Debug().Msg("ProxyPort no setted setting default 18080")
			config.ProxyPort = 18080
		}
		connFactory.TlsClientConfig = tlsConfig
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
