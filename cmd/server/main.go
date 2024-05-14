package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	log "log/slog"
	"minidocker/executor"
	"minidocker/pb"
	"minidocker/server"
	"minidocker/signal"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var listenAdd = flag.String("listen", "localhost:8080", "Listen address in host:port format")
var caChainFile = flag.String("ca", "ca/ca.crt", "CA location")
var certFile = flag.String("cert", "ca/server.crt", "Client certificate location")
var privateKeyFile = flag.String("key", "ca/server.key", "Server private key location")

func main() {
	flag.Parse()

	tlsConfig, tlsErr := LoadTlSConfig(*certFile, *privateKeyFile, *caChainFile)
	if tlsErr != nil {
		fmt.Println(tlsErr)
		os.Exit(1)
	}

	listener, err := net.Listen("tcp", *listenAdd)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}

	interceptor := server.NewRBACInterceptor()

	grpcServer := grpc.NewServer(
		grpc.Creds(tlsConfig),
		grpc.UnaryInterceptor(interceptor.UnaryInterceptor),
		grpc.StreamInterceptor(interceptor.StreamInterceptor),
		// We set a reasonable connection timeout for the type of service we have (short req/resp)
		// of course we have no data to define what reasonable is in a empirical or statical way.
		grpc.ConnectionTimeout(5*time.Second),
	)

	exec, err := executor.New()
	if err != nil {
		log.Error("failed to start executor", "error", err)
		os.Exit(1)
	}
	server := &server.SchedulerServer{Executor: exec}
	pb.RegisterSchedulerServer(grpcServer, server)

	signal.SetupSignalHandler(func(s os.Signal) {
		log.Info("Signal received, stopping server", "signal", s)
		grpcServer.GracefulStop()
	})

	exitCode := 0
	if err := grpcServer.Serve(listener); err != nil {
		log.Error("failed to start server", "error", err)
		exitCode = 1
	}

	log.Info("Terminated")
	os.Exit(exitCode)
}

func LoadTlSConfig(certFile, keyFile, caFile string) (credentials.TransportCredentials, error) {
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certification: %w", err)
	}

	data, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(data) {
		return nil, fmt.Errorf("unable to append the CA certificate to CA pool")
	}

	tlsConfig := &tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{certificate},
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS13,
	}
	return credentials.NewTLS(tlsConfig), nil
}
