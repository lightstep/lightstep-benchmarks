package clientlib

import (
	"fmt"
	"net"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/lightstep/lightstep-benchmarks/benchlib"

	proto_timestamp "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/lightstep/lightstep-tracer-go/collectorpb"
	"github.com/lightstep/lightstep-tracer-go/lightstep_thrift"
	"github.com/lightstep/lightstep-tracer-go/thrift_0_9_2/lib/go/thrift"
)

const (
	// collectorBinaryPath is the path of the Thrift collector service
	collectorBinaryPath = "/_rpc/v1/reports/binary"
	// collectorJSONPath is the path of the pure-JSON collector service
	collectorJSONPath = "/api/v0/reports"
)

type FakeCollector struct {
	processor        *lightstep_thrift.ReportingServiceProcessor
	processorFactory thrift.TProcessorFactory
	protocolFactory  thrift.TProtocolFactory

	grpcService struct {
	}

	spansReceived int64
	spansDropped  int64
	bytesReceived int64
}

func fakeReportResponse() *lightstep_thrift.ReportResponse {
	nowMicros := time.Now().UnixNano() / 1000
	return &lightstep_thrift.ReportResponse{Timing: &lightstep_thrift.Timing{&nowMicros, &nowMicros}}
}

func (fc *FakeCollector) Run() {
	fc.processor = lightstep_thrift.NewReportingServiceProcessor(fc)

	go fc.runGrpc()

}

// Report is a Thrift Collector method.
func (fc *FakeCollector) Report(auth *lightstep_thrift.Auth, request *lightstep_thrift.ReportRequest) (
	r *lightstep_thrift.ReportResponse, err error) {
	fc.spansReceived += int64(len(request.SpanRecords))
	fc.countDroppedSpans(request)
	return fakeReportResponse(), nil
}

func (fc *FakeCollector) countDroppedSpans(request *lightstep_thrift.ReportRequest) {
	if request.InternalMetrics == nil {
		return
	}
	for _, c := range request.InternalMetrics.Counts {
		if c.Name == "spans.dropped" {
			if c.Int64Value != nil {
				fc.spansDropped += *c.Int64Value
			} else if c.DoubleValue != nil {
				fc.spansDropped += int64(*c.DoubleValue)
			}
		}
	}
}

// Note: This is a duplicate of countDroppedSpans for a protobuf
// Report instead of a Thrift report.
func (fc *FakeCollector) countGrpcDroppedSpans(request *collectorpb.ReportRequest) {
	if request.InternalMetrics == nil {
		return
	}
	for _, c := range request.InternalMetrics.Counts {
		if c.Name == "spans.dropped" {
			switch t := c.Value.(type) {
			case *collectorpb.MetricsSample_IntValue:
				fc.spansDropped += t.IntValue
			case *collectorpb.MetricsSample_DoubleValue:
				fc.spansDropped += int64(t.DoubleValue)
			}
		}
	}
}

// BytesReceived is called from the HTTP layer before Thrift
// processing, recording inbound byte count.
func (fc *FakeCollector) BytesReceived(num int64) {
	fc.bytesReceived += num
}

func (fc *FakeCollector) grpcShim() *grpcService {
	return &grpcService{fc}
}

func (fc *FakeCollector) runGrpc() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", benchlib.GrpcPort))
	if err != nil {
		panic(fmt.Errorf("failed to listen: %v", err))
	}
	grpcServer := grpc.NewServer()

	collectorpb.RegisterCollectorServiceServer(grpcServer, fc.grpcShim())
	benchlib.Fatal(grpcServer.Serve(lis))
}

type grpcService struct {
	fakeCollector *FakeCollector
}

func (g *grpcService) Report(ctx context.Context, req *collectorpb.ReportRequest) (resp *collectorpb.ReportResponse, err error) {
	g.fakeCollector.spansReceived += int64(len(req.Spans))
	g.fakeCollector.countGrpcDroppedSpans(req)
	now := time.Now()
	ts := &proto_timestamp.Timestamp{
		Seconds: now.Unix(),
		Nanos:   int32(now.Nanosecond()),
	}
	return &collectorpb.ReportResponse{ReceiveTimestamp: ts, TransmitTimestamp: ts}, nil
}
