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
	"crypto/tls"
	"crypto/x509"
	"errors"
	pool "github.com/jolestar/go-commons-pool/v2"
	"github.com/rs/zerolog/log"
	"net"
	"os"
	"strconv"
	"sync/atomic"
	"unsafe"
)

type Connection struct {
	ConnectionToken *C.CTG_ConnToken_t
	Config          *ConnectionConfig
}

type ConnectionFactory struct {
	Config *ConnectionConfig
}

func (f *ConnectionFactory) MakeObject(ctx context.Context) (*pool.PooledObject, error) {
	ptr := C.malloc(C.sizeof_char * 1024)
	C.memset(ptr, C.int(C.sizeof_char*1024), 0)
	err := f.getCicsServer((*C.CTG_ConnToken_t)(ptr))
	if err != nil {
		return nil, err
	}
	return pool.NewPooledObject(
			&Connection{
				ConnectionToken: (*C.CTG_ConnToken_t)(ptr),
				Config:          f.Config,
			}),
		nil
}

func (f *ConnectionFactory) DestroyObject(ctx context.Context, object *pool.PooledObject) error {

	o := object.Object.(*Connection)
	closeGatewayConnection(o.ConnectionToken)
	defer C.free(unsafe.Pointer(o.ConnectionToken))
	return nil
}

func (f *ConnectionFactory) ValidateObject(ctx context.Context, object *pool.PooledObject) bool {
	return true
}

func (f *ConnectionFactory) ActivateObject(ctx context.Context, object *pool.PooledObject) error {
	return nil
}

func (f *ConnectionFactory) PassivateObject(ctx context.Context, object *pool.PooledObject) error {
	return nil
}

func (f *ConnectionFactory) getCicsServer(ptr *C.CTG_ConnToken_t) error {
	var hostname *C.char
	var port C.int
	if f.Config.UseProxy {
		hostname = C.CString("127.0.0.1")
		port = C.int(f.Config.ProxyPort)
	} else {
		hostname = C.CString(f.Config.Hostname)
		port = C.int(f.Config.Port)
	}
	defer C.free(unsafe.Pointer(hostname))
	ctgrg := C.CTG_openRemoteGatewayConnection(hostname, port, ptr, C.int(f.Config.Timeout))
	if ctgrg == C.CTG_OK {
		log.Info().Msg("Connessione CICS Avvenuta con successo")
		return nil
	} else {
		displayRc(ctgrg)
		return errors.New("Errore connessione CTG")
	}

}

func closeGatewayConnection(gatewayTokenPtr *C.CTG_ConnToken_t) {

	/* Close connection to CICS TG */
	ctgRc := C.CTG_closeGatewayConnection(gatewayTokenPtr)

	if ctgRc != C.CTG_OK {
		log.Info().Msg("Failed to close connection to CICS Transaction Gateway")

	} else {
		log.Info().Msg("Closed connection to CICS Transaction Gateway")
	}

}

func Encrypt(connectionConfig *ConnectionConfig, ready chan bool, errCh chan<- error) {
	localAddr := "127.0.0.1:" + strconv.Itoa(connectionConfig.ProxyPort)
	log.Info().Msgf("Listening: %v\nProxying & Encrypting: %v\n\n", localAddr, connectionConfig.Hostname+":"+strconv.Itoa(connectionConfig.Port))
	log.Info().Msgf("Reading %v as certificate, %v as key and %v as root certificate", connectionConfig.SSLClientCertificate, connectionConfig.SSLClientKey, connectionConfig.SSLRootCaCertificate)
	cert, err := tls.LoadX509KeyPair(connectionConfig.SSLClientCertificate, connectionConfig.SSLClientKey)
	caCert, err := os.ReadFile(connectionConfig.SSLRootCaCertificate)
	if err != nil {
		log.Fatal().Err(err).Msgf("failed to load cert: %s", err.Error())
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		InsecureSkipVerify: connectionConfig.InsecureSkipVerify,
		Certificates:       []tls.Certificate{cert}, // this certificate is used to sign the handshake
		RootCAs:            caCertPool,              // this is used to validate the server certificate
	}
	listener, err := net.Listen("tcp", localAddr)
	if err != nil {
		errCh <- err
		return
	}
	ready <- true
	defer func() {
		log.Info().Msg("Closing Proxy Socket")
		listener.Close()
	}()
	var ops uint64
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Error().Err(err).Msgf("error accepting connection %s", err.Error())
			continue
		}
		go startListener(connectionConfig, ops, tlsConfig, conn, errCh)

	}
}

func startListener(connectionConfig *ConnectionConfig, ops uint64, tlsConfig *tls.Config, conn net.Conn, errCh chan<- error) {

	i := atomic.AddUint64(&ops, 1)
	conn2, err := tls.Dial("tcp", connectionConfig.Hostname+":"+strconv.Itoa(connectionConfig.Port), tlsConfig)
	defer conn.Close()
	defer conn2.Close()

	if err != nil {
		log.Error().Err(err).Msgf("error dialing remote addr %s", err.Error())
		errCh <- err
		return
	}

	err = conn2.Handshake()
	if err != nil {
		log.Error().Err(err).Msgf("failed to complete handshake: %s\n", err.Error())
		errCh <- err
		return
	}
	log.Info().Msgf("%d connect [%s -> %s]", i, conn2.LocalAddr(), conn2.RemoteAddr())

	if len(conn2.ConnectionState().PeerCertificates) > 0 {
		log.Info().Msgf("client common name: %+v", conn2.ConnectionState().PeerCertificates[0].Subject.CommonName)
	}
	Pipe(conn, conn2)
	return
}

func chanFromConn(conn net.Conn) chan []byte {
	c := make(chan []byte)
	go func() {
		b := make([]byte, 1024)
		for {
			n, err := conn.Read(b)
			if n > 0 {
				res := make([]byte, n)
				copy(res, b[:n])
				c <- res
			}
			if err != nil {
				c <- nil
				break
			}

		}
	}()
	return c
}

func Pipe(conn1 net.Conn, conn2 net.Conn) {
	chan1 := chanFromConn(conn1)
	chan2 := chanFromConn(conn2)
	for {
		select {
		case b1 := <-chan1:
			if b1 == nil {
				return
			} else {
				conn2.Write(b1)
			}
		case b2 := <-chan2:
			if b2 == nil {
				return
			} else {
				conn1.Write(b2)
			}
		}
	}
}
