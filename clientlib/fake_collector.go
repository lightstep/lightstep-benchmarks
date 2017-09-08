package clientlib

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	proto_timestamp "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/lightstep/lightstep-tracer-go/collectorpb"
	"github.com/lightstep/lightstep-tracer-go/lightstep_thrift"
	"github.com/lightstep/lightstep-tracer-go/thrift_0_9_2/lib/go/thrift"
)

type ThriftHTTPTransport struct {
	io.ReadCloser
	io.Writer
}

type ThriftStream interface {
	BytesReceived(num int64)
}

type ThriftFactories struct {
	thrift.TProcessorFactory
	thrift.TProtocolFactory
	ThriftStream
}

func (p *ThriftHTTPTransport) Open() error  { return nil }
func (p *ThriftHTTPTransport) IsOpen() bool { return true }
func (p *ThriftHTTPTransport) Flush() error { return nil }

// ServeThriftHTTP is boilerplate for a Thrift connection (binary)
// with additional instrumentation for benchmarking purposes.
func (t *ThriftFactories) ServeThriftHTTP(res http.ResponseWriter, req *http.Request) {
	wrbuffer := bytes.NewBuffer(nil)
	rdbuffer := bytes.NewBuffer(nil)
	rdbytes, err := rdbuffer.ReadFrom(req.Body)
	if err != nil {
		fmt.Println("Could not read body: ", err)
	}

	client := &ThriftHTTPTransport{ioutil.NopCloser(rdbuffer), wrbuffer}

	t.ThriftStream.BytesReceived(rdbytes)

	tprocessor := t.GetProcessor(client)
	tprotocol := t.GetProtocol(client)

	ok, err := tprocessor.Process(tprotocol, tprotocol)

	if err != nil {
		fmt.Println("RPC Error: ", err)
	} else if !ok {
		fmt.Println("RPC request failed")
	}

	res.Header().Set("Content-Type", "application/octet-stream")

	if _, err := io.Copy(res, wrbuffer); err != nil {
		fmt.Println("ResponseWriter.Write", err)
	}
}

const (
	// collectorBinaryPath is the path of the Thrift collector service
	collectorBinaryPath = "/_rpc/v1/reports/binary"
	// collectorJSONPath is the path of the pure-JSON collector service
	collectorJSONPath = "/api/v0/reports"

	collectorHost = "localhost"
)

type FakeCollector struct {
	processor        *lightstep_thrift.ReportingServiceProcessor
	processorFactory thrift.TProcessorFactory
	protocolFactory  thrift.TProtocolFactory

	requestCh chan sreq

	grpcServer *grpc.Server

	spansReceived int64
	spansDropped  int64
	bytesReceived int64
}

func CreateFakeCollector() *FakeCollector {
	return &FakeCollector{}
}

func fakeReportResponse() *lightstep_thrift.ReportResponse {
	nowMicros := time.Now().UnixNano() / 1000
	return &lightstep_thrift.ReportResponse{Timing: &lightstep_thrift.Timing{&nowMicros, &nowMicros}}
}

func (fc *FakeCollector) Run(collectorHTTPPort *int, collectorGrpcPort *int) {

	if collectorGrpcPort != nil {
		go fc.runGrpc(*collectorGrpcPort)
	}

	if collectorHTTPPort != nil {
		fc.runThrift(*collectorHTTPPort)
	}
}

func (fc *FakeCollector) Stop() {
	fc.stopGrpc()
}

func (fc *FakeCollector) GetStats() (int64, int64, int64) {
	return fc.spansReceived, fc.spansDropped, fc.bytesReceived
}

func (fc *FakeCollector) ResetStats() {
	fc.spansReceived = 0
	fc.spansDropped = 0
	fc.bytesReceived = 0
}

func (fc *FakeCollector) runThrift(controllerHTTPPort int) {
	fc.processor = lightstep_thrift.NewReportingServiceProcessor(fc)

	address := fmt.Sprintf(":%v", controllerHTTPPort)
	mux := http.NewServeMux()

	tfactories := ThriftFactories{
		thrift.NewTProcessorFactory(fc.processor),
		thrift.NewTBinaryProtocolFactoryDefault(),
		fc}

	// Note: the 100000 second timeout avoids HTTP disconnections,
	// which can confuse very simple HTTP libraries (e.g., the C++
	// benchmark client).
	server := &http.Server{
		Addr:         address,
		ReadTimeout:  100000 * time.Second,
		WriteTimeout: 0 * time.Second,
		Handler:      http.HandlerFunc(fc.serializeHTTP),
	}

	mux.HandleFunc(collectorBinaryPath, tfactories.ServeThriftHTTP)
	mux.HandleFunc(collectorJSONPath, fc.ServeJSONHTTP)

	go func() {
		for req := range fc.requestCh {
			mux.ServeHTTP(req.w, req.r)
			close(req.doneCh)
		}
	}()
	go func() {
		panic(server.ListenAndServe())
	}()

}

func (fc *FakeCollector) serializeHTTP(w http.ResponseWriter, r *http.Request) {
	doneCh := make(chan struct{})
	fc.requestCh <- sreq{w, r, doneCh}
	<-doneCh
}

// ServeJSONHTTP is more-or-less copied from crouton/cmd/collector/main.go
func (fc *FakeCollector) ServeJSONHTTP(res http.ResponseWriter, req *http.Request) {
	// Support the "Content-Encoding: gzip" if it's there
	var bodyReader io.ReadCloser
	switch req.Header.Get("Content-Encoding") {
	case "gzip":
		var err error
		bodyReader, err = gzip.NewReader(req.Body)
		if err != nil {
			http.Error(res, fmt.Sprintf("Could not decode gzipped content"),
				http.StatusBadRequest)
			return
		}
		defer bodyReader.Close()
	default:
		bodyReader = req.Body
	}

	body, err := ioutil.ReadAll(bodyReader)
	if err != nil {
		http.Error(res, "Unable to read body: "+err.Error(), http.StatusBadRequest)
		return
	}

	reportRequest := &lightstep_thrift.ReportRequest{}
	if err := json.Unmarshal(body, reportRequest); err != nil {
		http.Error(res, "Unable to decode body: "+err.Error(), http.StatusBadRequest)
		return
	}

	fc.spansReceived += int64(len(reportRequest.SpanRecords))
	fc.bytesReceived += int64(len(body))

	fc.countDroppedSpans(reportRequest)

	res.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(res).Encode(fakeReportResponse()); err != nil {
		http.Error(res, "Unable to encode response: "+err.Error(), http.StatusBadRequest)
	}
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

func (fc *FakeCollector) runGrpc(grpcPort int) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%v", grpcPort))
	if err != nil {
		panic(fmt.Errorf("failed to listen: %v", err))
	}
	fc.grpcServer = grpc.NewServer()

	collectorpb.RegisterCollectorServiceServer(fc.grpcServer, fc.grpcShim())
	err = fc.grpcServer.Serve(lis)
	if err != nil {
		// TODO Handle error grpc server close
		fmt.Println("GRPC Server error: ", err)
	}
}

func (fc *FakeCollector) stopGrpc() {
	fc.grpcServer.GracefulStop()
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
