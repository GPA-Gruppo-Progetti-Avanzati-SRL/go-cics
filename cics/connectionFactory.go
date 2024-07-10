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
	Port            int
	Tunnel          chan struct{}
}

type ConnectionFactory struct {
	Config   *ConnectionConfig
	PortPool *PortPool
}

func (f *ConnectionFactory) MakeObject(ctx context.Context) (*pool.PooledObject, error) {
	ptr := C.malloc(C.sizeof_char * 1024)
	C.memset(ptr, C.int(C.sizeof_char*1024), 0)
	connection := &Connection{
		Config: f.Config,
	}
	port := 0
	if f.Config.UseProxy {
		port = f.PortPool.GetPort()
		connection.Port = port
		tunnel := make(chan struct{})
		err := f.getCicsServerUsingProxy(port, (*C.CTG_ConnToken_t)(ptr), tunnel)
		if err != nil {
			return nil, err
		}
		connection.ConnectionToken = (*C.CTG_ConnToken_t)(ptr)
		connection.Tunnel = tunnel
	} else {
		err := f.getCicsServer((*C.CTG_ConnToken_t)(ptr))
		if err != nil {
			return nil, err
		}
		connection.ConnectionToken = (*C.CTG_ConnToken_t)(ptr)
	}

	return pool.NewPooledObject(

			connection),
		nil
}

func (f *ConnectionFactory) DestroyObject(ctx context.Context, object *pool.PooledObject) error {
	log.Debug().Msg("Destroy connection")
	o := object.Object.(*Connection)
	f.closeGatewayConnection(o.ConnectionToken)
	if f.Config.UseProxy {
		close(o.Tunnel)
		f.PortPool.ReleasePort(o.Port)
	}
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

func (f *ConnectionFactory) getCicsServerUsingProxy(aPort int, ptr *C.CTG_ConnToken_t, tunnel chan struct{}) error {

	hostname := C.CString("127.0.0.1")
	port := C.int(aPort)
	proxyready := make(chan bool)
	errCh := make(chan error, 3)
	go f.Encrypt(aPort, proxyready, errCh, tunnel)
	log.Info().Msg("Wait opening socket")
	<-proxyready
	for err := range errCh {
		if err != nil {
			log.Error().Msgf("Proxy %s", err)
			return err
		}
	}
	log.Info().Msg("Socket opened")

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

func (f *ConnectionFactory) getCicsServer(ptr *C.CTG_ConnToken_t) error {
	var hostname *C.char
	var port C.int

	hostname = C.CString(f.Config.Hostname)
	port = C.int(f.Config.Port)

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

func (f *ConnectionFactory) closeGatewayConnection(gatewayTokenPtr *C.CTG_ConnToken_t) {

	/* Close connection to CICS TG */
	ctgRc := C.CTG_closeGatewayConnection(gatewayTokenPtr)

	if ctgRc != C.CTG_OK {
		log.Info().Msg("Failed to close connection to CICS Transaction Gateway")

	} else {
		log.Info().Msg("Closed connection to CICS Transaction Gateway")
	}

}

func (f *ConnectionFactory) Encrypt(aPort int, ready chan bool, errCh chan<- error, tunnel chan struct{}) {
	localAddr := "127.0.0.1:" + strconv.Itoa(aPort)
	log.Info().Msgf("Listening: %v\nProxying & Encrypting: %v\n\n", localAddr, f.Config.Hostname+":"+strconv.Itoa(aPort))
	log.Info().Msgf("Reading %v as certificate, %v as key and %v as root certificate", f.Config.SSLClientCertificate, f.Config.SSLClientKey, f.Config.SSLRootCaCertificate)
	cert, err := tls.LoadX509KeyPair(f.Config.SSLClientCertificate, f.Config.SSLClientKey)
	caCert, err := os.ReadFile(f.Config.SSLRootCaCertificate)
	if err != nil {
		log.Fatal().Err(err).Msgf("failed to load cert: %s", err.Error())
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		InsecureSkipVerify: f.Config.InsecureSkipVerify,
		Certificates:       []tls.Certificate{cert}, // this certificate is used to sign the handshake
		RootCAs:            caCertPool,              // this is used to validate the server certificate
	}
	listener, err := net.Listen("tcp", localAddr)
	if err != nil {
		errCh <- err
		ready <- false
		return
	}
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
		go f.startListener(ops, tlsConfig, conn, errCh, ready, tunnel)

	}
}

func (f *ConnectionFactory) startListener(ops uint64, tlsConfig *tls.Config, conn net.Conn, errCh chan<- error, ready chan bool, tunnel chan struct{}) {

	i := atomic.AddUint64(&ops, 1)
	conn2, err := tls.Dial("tcp", f.Config.Hostname+":"+strconv.Itoa(f.Config.Port), tlsConfig)
	defer conn.Close()
	defer conn2.Close()

	if err != nil {
		log.Error().Err(err).Msgf("error dialing remote addr %s", err.Error())
		errCh <- err
		ready <- false
		return
	}

	err = conn2.Handshake()
	if err != nil {
		log.Error().Err(err).Msgf("failed to complete handshake: %s\n", err.Error())
		errCh <- err
		ready <- false
		return
	}
	log.Info().Msgf("%d connect [%s -> %s]", i, conn2.LocalAddr(), conn2.RemoteAddr())

	if len(conn2.ConnectionState().PeerCertificates) > 0 {
		log.Info().Msgf("client common name: %+v", conn2.ConnectionState().PeerCertificates[0].Subject.CommonName)
	}
	Pipe(conn, conn2, tunnel)
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

func Pipe(conn1 net.Conn, conn2 net.Conn, tunnel chan struct{}) {
	chan1 := chanFromConn(conn1)
	chan2 := chanFromConn(conn2)
	for {
		select {
		case <-tunnel:
			log.Debug().Msg("Ricevuta chiusura canale")
			return
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
