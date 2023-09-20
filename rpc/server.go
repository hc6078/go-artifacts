package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	grecover "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/hc6078/go-artifacts/sig"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"net"
	"runtime"
	"strconv"
	"sync"
)

// GrpcServerInstance GRPC 实例
type GrpcServerInstance struct {
	*grpc.Server
	// 端口
	port int
	// 开启监控
	metric bool
}

type Option func(s *GrpcServerInstance)

func WithPort(port int) Option {
	return func(s *GrpcServerInstance) {
		s.port = port
	}
}

// GracefulClose 实现优雅退出
func (s *GrpcServerInstance) GracefulClose(ctx context.Context, signal *sig.SignalFlag) {
	glog.Info(ctx, "grpc server waiting close")
	s.Server.GracefulStop()
	glog.Info(ctx, "grpc server closed")
}

func (s *GrpcServerInstance) RecoverFunc(ctx context.Context, err error) {
	glog.Error(ctx, err)
}

var grpcServer *GrpcServerInstance

var grpcOnce sync.Once

// GetGrpcServerInstance 获取 grpc 服务
func GetGrpcServerInstance() *GrpcServerInstance {
	grpcOnce.Do(func() {
		grpcServer = &GrpcServerInstance{
			Server: grpc.NewServer(registerMiddleware()...),
		}
	})
	return grpcServer
}

// Run 服务运行
func (s *GrpcServerInstance) Run(ctx context.Context, f func(server *grpc.Server), options ...Option) {
	for _, optionFunc := range options {
		optionFunc(s)
	}
	if s.port == 0 {
		s.port = 8080
	}
	l, err := net.Listen("tcp", ":"+strconv.Itoa(s.port))
	if err != nil {
		glog.Fatalf(err.Error())
	}
	f(s.Server)
	// 注册优雅退出
	sig.Get().RegisterClose(s)
	go func() {
		if err := s.Server.Serve(l); err != nil {
			glog.Fatalf(err.Error())
		}
	}()
	glog.Info(ctx, "grpc 服务启动成功,端口监听:", s.port)
}

type loggerUploadMessage struct {
	TraceID string      `json:"traceID"`
	Method  string      `json:"method"`
	Req     interface{} `json:"req"`
	ReqTime float64     `json:"request_time"`
	Error   string      `json:"error"`
}

// ToString 日志转换
func (s *loggerUploadMessage) ToString() string {
	body, _ := json.Marshal(s)
	return string(body)
}

// trace 请求链路透传 ctx
func trace() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (
		resp interface{}, err error) {
		md, _ := metadata.FromIncomingContext(ctx)
		metaTraceID := md.Get(TraceIDFlag)
		if len(metaTraceID) > 0 {
			ctx = context.WithValue(ctx, TraceIDFlag, metaTraceID[0])
		}
		resp, err = handler(ctx, req)
		return resp, err
	}
}

// registerMiddleware 中间件
func registerMiddleware() []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			trace(),
			grecover.UnaryServerInterceptor(recoveryInterceptor()),
		),
		grpc.WriteBufferSize(0), grpc.ReadBufferSize(0),
		grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
		grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()),
	}
}

// recoveryInterceptor
func recoveryInterceptor() grecover.Option {
	ctx := context.Background()
	return grecover.WithRecoveryHandler(func(p interface{}) (err error) {
		buf := make([]byte, 4096)
		num := runtime.Stack(buf, false)
		msg := fmt.Sprintf("[grpc_panic_recovery]: %v %s", p, string(buf[:num]))
		glog.Error(ctx, msg)
		return status.Errorf(codes.Unknown, "%s", msg)
	})
}
