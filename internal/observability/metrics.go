package observability

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chat_http_requests_total",
			Help: "Total number of HTTP requests processed by the chat service.",
		},
		[]string{"method", "route", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "chat_http_request_duration_seconds",
			Help:    "HTTP request latencies in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"route"},
	)
	grpcServerHandledTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_server_handled_total",
			Help: "Total number of gRPC requests handled by the server.",
		},
		[]string{"grpc_service", "grpc_method", "grpc_code"},
	)
	wsActiveConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "chat_ws_active_connections",
			Help: "Number of active websocket connections.",
		},
		[]string{"kind"},
	)
	wsEventsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chat_ws_events_total",
			Help: "Total number of websocket events.",
		},
		[]string{"kind", "event"},
	)
	amqpPublishErrorsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "chat_amqp_publish_errors_total",
			Help: "Total number of AMQP publish errors.",
		},
	)
)

func init() {
	prometheus.MustRegister(
		httpRequestsTotal,
		httpRequestDuration,
		grpcServerHandledTotal,
		wsActiveConnections,
		wsEventsTotal,
		amqpPublishErrorsTotal,
	)
}

func HTTPMetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		status := c.Writer.Status()

		httpRequestsTotal.WithLabelValues(c.Request.Method, route, strconv.Itoa(status)).Inc()
		httpRequestDuration.WithLabelValues(route).Observe(time.Since(start).Seconds())
	}
}

func GRPCServerMetricsUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		resp, err := handler(ctx, req)
		statusInfo := status.Convert(err)
		service, method := splitFullMethod(info.FullMethod)
		grpcServerHandledTotal.WithLabelValues(service, method, statusInfo.Code().String()).Inc()
		return resp, err
	}
}

func splitFullMethod(fullMethod string) (string, string) {
	parts := strings.Split(fullMethod, "/")
	if len(parts) < 3 {
		return "unknown", "unknown"
	}
	return parts[1], parts[2]
}

func IncWSActive(kind string) {
	wsActiveConnections.WithLabelValues(kind).Inc()
}

func DecWSActive(kind string) {
	wsActiveConnections.WithLabelValues(kind).Dec()
}

func IncWSEvent(kind, event string) {
	wsEventsTotal.WithLabelValues(kind, event).Inc()
}

func IncAMQPPublishError() {
	amqpPublishErrorsTotal.Inc()
}
