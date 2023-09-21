package rpc

import (
	"context"
	"fmt"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/vrieske/go-artifacts/sig"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"sync"
)

const TraceIDFlag = "trace-id"

const GrpcTraceFlag = "grpc-trace-flag"

var clientWg = new(sync.WaitGroup)

var grpcOnceClient sync.Once

type gracefulClient struct {
}

// 客户端优雅退出
func (s *gracefulClient) GracefulClose(ctx context.Context, signal *sig.SignalFlag) {
	clientWg.Wait()
}

func (s *gracefulClient) RecoverFunc(ctx context.Context, err error) {
	glog.Error(ctx, err)
}

// invoke 调用方
func invoke() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		clientWg.Add(1)
		defer clientWg.Done()
		var traceID string
		if traceID = gconv.String(ctx.Value(TraceIDFlag)); traceID != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, TraceIDFlag, traceID)
		}
		start := gtime.Now().UnixMilli()
		p := &loggerUploadMessage{
			TraceID: traceID,
			Method:  method,
			Req:     req,
		}
		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			p.Error = err.Error()
		}
		end := gtime.Now().UnixMilli()
		p.ReqTime = gconv.Float64(fmt.Sprintf("%.2f", (float64(end)-float64(start))/1000))
		ctx = context.WithValue(ctx, GrpcTraceFlag, GrpcTraceFlag)
		ctx = context.WithValue(ctx, TraceIDFlag, traceID)
		glog.Info(ctx, p.ToString())
		return err
	}
}

// NewGrpcClient 启动一个 Grpc 客户端
func NewGrpcClient(ctx context.Context, svc string) *grpc.ClientConn {
	grpcOnceClient.Do(func() {
		sig.Get().RegisterClose(&gracefulClient{})
	})
	conn, err := grpc.Dial(svc,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			invoke()),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
	)
	if err != nil {
		glog.Fatalf(ctx, "grpc客户端连接失败 %s", err)
	}
	return conn
}
