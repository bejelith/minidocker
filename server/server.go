package server

import (
	"context"
	"fmt"
	"io"
	log "log/slog"
	"minidocker/executor"
	"minidocker/pb"
	"strings"
)

type SchedulerServer struct {
	pb.UnimplementedSchedulerServer
	Executor *executor.Executor
}

func (s *SchedulerServer) Get(ctx context.Context, r *pb.GetRequest) (*pb.GetResponse, error) {
	p := s.Executor.Get(uint64(r.Pid))
	if p == nil {
		return &pb.GetResponse{Found: false, Pid: 0}, nil
	}
	return &pb.GetResponse{Found: true, Pid: p.ID, Status: p.State}, nil
}

func (s *SchedulerServer) Start(ctx context.Context, r *pb.CreateRequest) (*pb.CreateResponse, error) {
	var errorStr string
	pid, err := s.Executor.Start(&executor.ProcessConfig{Cmd: r.Cmd, Args: r.Args})
	if err != nil {
		log.Warn("command execution failed", "command", r.Cmd, "args", strings.Join(r.Args, " "))
		errorStr = err.Error()
		return &pb.CreateResponse{Pid: pid, Error: &errorStr}, nil
	}
	return &pb.CreateResponse{Pid: pid, Error: nil}, nil
}

func (s *SchedulerServer) Stdout(r *pb.OutputRequest, stream pb.Scheduler_StdoutServer) error {
	job := s.Executor.Get(r.Pid)
	if job == nil {
		return fmt.Errorf("job does not exists")
	}
	reader, _ := s.Executor.Stdout(job.ID)

	var buffer = make([]byte, 1024)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			if sendErr := stream.Send(&pb.OutputResponse{Output: buffer[:n]}); sendErr != nil {
				log.Warn("error writing to stream", "process", r.Pid, "error", sendErr)
				break
			}
		}
		if err != nil {
			if err != io.EOF {
				log.Warn("error reading from stream", "process", r.Pid, "error", err)
			}
			break
		}
	}
	return nil
}

func (s *SchedulerServer) Stop(ctx context.Context, r *pb.StopRequest) (*pb.StopResponse, error) {
	s.Executor.StopProcess(r.Pid)
	return &pb.StopResponse{}, nil
}
