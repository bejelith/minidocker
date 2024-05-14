package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"minidocker/pb"
	"minidocker/signal"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var runFlags = flag.NewFlagSet("run", flag.ExitOnError)
var processCPU = runFlags.Uint("cpu", 10, "Set process maximum cpu usage as percentage")
var processMEM = runFlags.Uint("mem", 1024, "Set process maximum memory expressed in MB")
var processRBPS = runFlags.Uint("rbps", 10, "Set process maximum read speed in bytes/s")
var processWBPS = runFlags.Uint("wbps", 10, "Set process maximum write speed in bytes/s")

var commonFlags = flag.NewFlagSet("common", flag.ExitOnError)
var serverAddr = commonFlags.String("addr", "localhost:8080", "Server address in host:port format")
var caFile = commonFlags.String("ca", "ca/ca.crt", "CA chain certificate location")
var certFile = commonFlags.String("cert", "ca/client_user1.crt", "Client certificate location")
var privateKeyFile = commonFlags.String("key", "ca/client_user1.key", "Client private key location")

// NOTE: I would have used cobra commands parsing library to managed sub-commands flags if given the choice.
func main() {

	commonFlags.Usage = func() {
		fmt.Fprintf(os.Stderr,
			"Usage:\n"+
				"\t%s [flags] command\nCommands:\n"+
				"\t- run [run flags] executable [args]\n"+
				"\t- output pid\n"+
				"\t- stop pid\n"+
				"Flags:\n",
			filepath.Base(os.Args[0]))
		commonFlags.PrintDefaults()
	}
	if commonFlags.Parse(os.Args[1:]) != nil {
		commonFlags.Usage()
		os.Exit(1)
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	signal.SetupSignalHandler(func(_ os.Signal) {
		cancelFunc()
		<-ctx.Done()
	})

	command := commonFlags.Arg(0)
	var commandError error
	switch command {
	case "get":
		// Print usage and exit if PID is not preset or not an integer
		pid, err := strconv.Atoi(commonFlags.Arg(1))
		if err != nil {
			fmt.Printf("could not parse PID \"%s\":%v\n", commonFlags.Arg(1), err)
			return
		}
		client := buildSchedulerClient()
		commandError = get(ctx, client, uint64(pid))
	case "run":
		runFlags.Usage = func() {
			fmt.Println("run command flags:")
			runFlags.PrintDefaults()
		}

		// Print usage and exit if there is an error parsing runFlags
		if err := runFlags.Parse(commonFlags.Args()[1:]); err != nil {
			fmt.Println(err)
			runFlags.Usage()
			os.Exit(1)
		}

		// No arguments passed to run command so we print error and exit
		if len(runFlags.Args()) == 0 {
			fmt.Println("No arguments to command run")
			runFlags.Usage()
			os.Exit(1)
		}

		var args []string
		// Get additional command arguments if present
		if len(runFlags.Args()) > 1 {
			args = append(args, runFlags.Args()[1:]...)
		}

		client := buildSchedulerClient()
		commandError = run(ctx, client, runFlags.Arg(0), args)
	case "output":
		pid, err := strconv.Atoi(commonFlags.Arg(1))
		if err != nil {
			fmt.Printf("could not parse PID \"%s\":%v\n", commonFlags.Arg(1), err)
			return
		}
		client := buildSchedulerClient()
		commandError = output(ctx, client, uint64(pid))
		// We want to ignore context cancelled errors as it's expected when using SIGTERM
		if commandError == context.Canceled {
			commandError = nil
		}
	default:
		commonFlags.Usage()
		os.Exit(1)
	}

	if commandError != nil {
		cancelFunc()
		fmt.Println(commandError)
		os.Exit(1)
	}
}

func get(ctx context.Context, c pb.SchedulerClient, p uint64) error {
	r, err := c.Get(ctx, &pb.GetRequest{Pid: p})
	if err != nil {
		return err
	}
	if !r.Found {
		return fmt.Errorf("pid %d does not exists", p)
	}
	fmt.Printf("PID: %d, Status: %s\n", r.Pid, r.Status)
	return nil
}

func output(ctx context.Context, c pb.SchedulerClient, p uint64) error {
	stdReader, err := c.Stdout(ctx, &pb.OutputRequest{Pid: p})
	if err != nil {
		return err
	}
	for {
		response, err := stdReader.Recv()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		fmt.Print(string(response.Output))
	}
}

// run executes the Executer Start command
func run(ctx context.Context, c pb.SchedulerClient, cmd string, args []string) error {
	limits := &pb.ResourceLimits{
		CpuPercentage: uint32(*processCPU),
		MemoryMB:      uint64(*processMEM),
		ReadBPS:       uint32(*processRBPS),
		WriteBPS:      uint32(*processWBPS),
	}

	r, err := c.Start(ctx, &pb.CreateRequest{Cmd: cmd, Args: args, Limits: limits})
	if err != nil {
		return err
	}

	if r.Error != nil {
		return errors.New(*r.Error)
	}

	fmt.Printf("Process ID: %d\n", r.Pid)
	return nil
}

func buildSchedulerClient() pb.SchedulerClient {
	conn, err := buildGRPCClient()
	if err != nil {
		fmt.Printf("client initialization error: %v\n", err)
		os.Exit(1)
	}
	client := pb.NewSchedulerClient(conn)

	signal.SetupSignalHandler(func(s os.Signal) {
		conn.Close()
	})
	return client
}

func buildGRPCClient() (*grpc.ClientConn, error) {
	ca_b, err := os.ReadFile(*caFile)
	if err != nil {
		return nil, fmt.Errorf("could not load CA certificate: %v", err)
	}
	caChain := x509.NewCertPool()
	caChain.AppendCertsFromPEM(ca_b)

	cert, err := tls.LoadX509KeyPair(*certFile, *privateKeyFile)
	if err != nil {
		log.Fatalf("failed to read client certificate: %v", err)
	}

	tlsCredentials := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caChain,
	})

	return grpc.Dial(*serverAddr, grpc.WithTransportCredentials(tlsCredentials))
}
