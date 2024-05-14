package server

import (
	"context"
	"fmt"
	"minidocker/pb"
	"reflect"
	"sync"

	log "log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

// PIDGetter matches all GRPC calls accessing by PID
type PIDGetter interface {
	GetPid() uint64
}

// PidGetter matches all GRPC all Start() calls
type CmdGetter interface {
	GetCmd() string
}

// wrappedStream is necessary to access the grpc message in order to authorize
// the request
type wrappedStream struct {
	grpc.ServerStream
	ctx  context.Context
	i    *RBACInterceptor
	user string
}

func (s *wrappedStream) Context() context.Context {
	return s.ctx
}

func (s *wrappedStream) RecvMsg(m any) error {
	switch msg := m.(type) {
	case *pb.OutputRequest:
		if s.i.VerifyOwnership(s.user, msg.Pid) {
			return s.ServerStream.RecvMsg(m)
		}
		return fmt.Errorf("user %s not authorized for pid %d", s.user, msg.Pid)
	default:
		return fmt.Errorf("user %s not authorized to this operation: %s", s.user, reflect.TypeOf(m))
	}
}

// RBACInterceptor implements grpc.UnaryInterceptor and grpc.StreamInterceptor allowing to
// allow or deny requests depending on the user role.
// Note: I overcomplicated the design because I wanted to have a dynamic way
// to add new API to the RBAC routines (see PidGetter/CmdGetter) rather then using
// the classic switch/if/else and command pattern approach.
// This works but the contract with the GRPC is not well designed but i'm sure
// there must be a way to enforce this at compile time.
// NOTE 2: not synchronizing on roles and user is ok was they never mutate.
type RBACInterceptor struct {
	users     map[string]string
	roles     map[string]map[string]struct{}
	userToPID map[string]map[uint64]struct{}
	mu        sync.RWMutex
}

// NewRBACInterceptor builds a new interceptor with a template user database.
func NewRBACInterceptor() *RBACInterceptor {
	// NOTE: This would be normally backed by an LDAP directory, i'm using hardcoded users for brevity.
	users := map[string]string{"user1": "admin", "user2": "user", "user3": "user"}
	roles := map[string]map[string]struct{}{
		"admin": {"*": {}},
		"user":  {"cat": {}, "ls": {}, "sleep": {}, "echo": {}},
	}
	return &RBACInterceptor{
		users:     users,
		roles:     roles,
		userToPID: map[string]map[uint64]struct{}{},
		mu:        sync.RWMutex{},
	}
}

// UnaryInterceptor verifies authorization for users accessing request/response API
func (i *RBACInterceptor) UnaryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, respErr error) {
	md, authErr := i.parseMetadata(ctx)
	if authErr != nil {
		log.Error("error getting user information", "error", authErr)
		return nil, fmt.Errorf("error identifying user: %v", authErr)
	}
	user := md["user"][0]
	role := md["role"][0]
	switch r := req.(type) {
	case CmdGetter:
		if !i.AuthorizeCmd(role, r.GetCmd()) {
			log.Warn("user unauthorized", "user", user, "role", role, "cmd", r.GetCmd())
			return nil, fmt.Errorf("user %s/%s not authorized to run %s", role, user, r.GetCmd())
		}
		resp, e := handler(ctx, req)
		pidResponse := resp.(PIDGetter)
		i.AttributeOwnership(user, uint64(pidResponse.GetPid()))

		//if log.Level() == log.LevelDebug {
		//
		log.Debug("command execution", "user", user, "role", role, "command", r.GetCmd())

		return resp, e
	case PIDGetter:
		if !i.VerifyOwnership(user, uint64(r.GetPid())) {
			log.Warn("user unauthorized", "user", user, "role", role, "process", r.GetPid())
			return nil, fmt.Errorf("user %s/%s not authorized", role, user)
		}
		//if log.Level() == log.LevelDebug {
		//
		log.Debug("reading process", "user", user, "role", role, "PID", r.GetPid())
		return handler(ctx, req)
	default:
		log.Error("unrecognized request", "user", user, "role", role, "request", reflect.TypeOf(req).String())
		return nil, fmt.Errorf("user %s/%s not authorized", role, user)
	}
}

// StreamInterceptor verifies authorization for users accessing streaming APIs
func (i *RBACInterceptor) StreamInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	md, err := i.parseMetadata(ss.Context())
	if err != nil {
		log.Error("error getting user information", "error", err)
		return fmt.Errorf("error identifying user: %v", err)
	}
	return handler(srv, &wrappedStream{ss, metadata.NewIncomingContext(ss.Context(), md), i, md["user"][0]})
}

// AuthorizeCmd verifies the role is allowed to execute the command specified
func (i *RBACInterceptor) AuthorizeCmd(role string, cmd string) bool {
	// Check if the role exists
	if _, exists := i.roles[role]; !exists {
		return false
	}
	// Check if the role has cmd as granted command
	if _, granted := i.roles[role][cmd]; granted {
		return true
	}
	return i.roleIsAdmin(role)
}

func (i *RBACInterceptor) roleIsAdmin(role string) bool {
	// Ultimately check if the user is admin
	_, isAdmin := i.roles[role]["*"]
	return isAdmin
}

// VerifyOwnership will return true if the user is operating on a Process it owns
func (i *RBACInterceptor) VerifyOwnership(user string, pid uint64) bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if pids, userExists := i.userToPID[user]; userExists {
		if _, isOwner := pids[pid]; isOwner {
			return true
		}
	}
	return i.userIsAdmin(user)
}

func (i *RBACInterceptor) userIsAdmin(user string) bool {
	// Ultimately check if the user is admin
	role, exists := i.users[user]
	if !exists {
		return false
	}
	return i.roleIsAdmin(role)
}

// AttributeOwnership updates the userToPID ownership map to determine which users started a process
func (i *RBACInterceptor) AttributeOwnership(user string, pid uint64) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if _, ok := i.userToPID[user]; !ok {
		i.userToPID[user] = map[uint64]struct{}{pid: {}}
	} else {
		i.userToPID[user][pid] = struct{}{}
	}
}

// parseMetadata will extract Metadata from GRPC context to retrieve the user and it's role
func (i *RBACInterceptor) parseMetadata(ctx context.Context) (metadata.MD, error) {
	if p, ok := peer.FromContext(ctx); ok {
		tlsInfo, found := p.AuthInfo.(credentials.TLSInfo)
		if !found {
			return nil, fmt.Errorf("no TLS info found")
		}
		user := tlsInfo.State.PeerCertificates[0].Subject.CommonName
		md, found := metadata.FromIncomingContext(ctx)
		if !found {
			md = metadata.MD{}
		}
		md.Append("user", user)
		md.Append("role", i.users[user])
		return md, nil
	}
	return nil, fmt.Errorf("no peer found from context")
}
