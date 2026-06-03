package middleware

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor returns a gRPC unary server interceptor that creates
// a span for each RPC and propagates trace context for downstream LLM calls.
//
// Usage:
//
//	s := grpc.NewServer(
//	    grpc.UnaryInterceptor(middleware.UnaryServerInterceptor("my-service")),
//	)
func UnaryServerInterceptor(serviceName string) grpc.UnaryServerInterceptor {
	tracer := otel.Tracer("go-ai-obs/grpc")
	propagator := propagation.TraceContext{}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// Extract trace context from incoming gRPC metadata
		md, _ := metadata.FromIncomingContext(ctx)
		ctx = propagator.Extract(ctx, metadataCarrier(md))

		// Start span for this RPC
		ctx, span := tracer.Start(ctx, info.FullMethod,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.RPCSystemGRPC,
				semconv.RPCService(info.FullMethod),
				attribute.String("service.name", serviceName),
			),
		)
		defer span.End()

		// Call handler
		resp, err := handler(ctx, req)

		// Record status
		if err != nil {
			st, _ := status.FromError(err)
			span.SetStatus(codes.Error, err.Error())
			span.SetAttributes(
				semconv.RPCGRPCStatusCodeKey.Int64(int64(st.Code())),
			)
		} else {
			span.SetStatus(codes.Ok, "")
		}

		return resp, err
	}
}

// StreamServerInterceptor returns a gRPC stream server interceptor that creates
// a span for each streaming RPC.
//
// Usage:
//
//	s := grpc.NewServer(
//	    grpc.StreamInterceptor(middleware.StreamServerInterceptor("my-service")),
//	)
func StreamServerInterceptor(serviceName string) grpc.StreamServerInterceptor {
	tracer := otel.Tracer("go-ai-obs/grpc")
	propagator := propagation.TraceContext{}

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()

		// Extract trace context
		md, _ := metadata.FromIncomingContext(ctx)
		ctx = propagator.Extract(ctx, metadataCarrier(md))

		// Start span
		ctx, span := tracer.Start(ctx, info.FullMethod,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.RPCSystemGRPC,
				semconv.RPCService(info.FullMethod),
				attribute.String("service.name", serviceName),
				attribute.Bool("grpc.stream", true),
			),
		)
		defer span.End()

		// Wrap stream with traced context
		wrappedStream := &tracedServerStream{
			ServerStream: ss,
			ctx:          ctx,
		}

		err := handler(srv, wrappedStream)
		if err != nil {
			st, _ := status.FromError(err)
			span.SetStatus(codes.Error, err.Error())
			span.SetAttributes(semconv.RPCGRPCStatusCodeKey.Int64(int64(st.Code())))
		} else {
			span.SetStatus(codes.Ok, "")
		}

		return err
	}
}

// tracedServerStream wraps grpc.ServerStream to inject the traced context.
type tracedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *tracedServerStream) Context() context.Context {
	return s.ctx
}

// metadataCarrier adapts gRPC metadata to the OTel propagation.TextMapCarrier interface.
type metadataCarrier metadata.MD

func (mc metadataCarrier) Get(key string) string {
	vals := metadata.MD(mc).Get(key)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func (mc metadataCarrier) Set(key, value string) {
	metadata.MD(mc).Set(key, value)
}

func (mc metadataCarrier) Keys() []string {
	md := metadata.MD(mc)
	keys := make([]string, 0, len(md))
	for k := range md {
		keys = append(keys, k)
	}
	return keys
}
