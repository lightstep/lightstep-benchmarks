// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: github.com/lightstep/lightstep-tracer-common/tmpgen/metrics.proto

/*
	Package metricspb is a generated protocol buffer package.

	It is generated from these files:
		github.com/lightstep/lightstep-tracer-common/tmpgen/metrics.proto

	It has these top-level messages:
		MetricPoint
		IngestRequest
		IngestResponse
*/
package metricspb // import "github.com/lightstep/lightstep-tracer-common/golang/gogo/metricspb"

import proto "github.com/gogo/protobuf/proto"
import fmt "fmt"
import math "math"
import lightstep_collector "github.com/lightstep/lightstep-tracer-common/golang/gogo/collectorpb"
import _ "google.golang.org/genproto/googleapis/api/annotations"
import google_protobuf2 "github.com/gogo/protobuf/types"
import google_protobuf "github.com/gogo/protobuf/types"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

import encoding_binary "encoding/binary"

import io "io"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion2 // please upgrade the proto package

// MetricKind indicates the semantics of the points (i.e. how to interpret values
// relative to each other).
type MetricKind int32

const (
	// InvalidMetricKind is the default value for the MetricKind. Some languages' proto compilers
	// (e.g. Go) return the default if no value is set. The default is marked invalid here to
	// avoid a common mistake where a field is left unset and appears to be set to the default.
	MetricKind_INVALID_METRIC_KIND MetricKind = 0
	// Counter metrics measure change over an interval.
	// When aggregated, counter metrics are usually added.
	MetricKind_COUNTER MetricKind = 1
	// Gauge metrics measure the value at a point in time.
	// When aggregated, intermediate values are often dropped for the latest value.
	MetricKind_GAUGE MetricKind = 2
)

var MetricKind_name = map[int32]string{
	0: "INVALID_METRIC_KIND",
	1: "COUNTER",
	2: "GAUGE",
}
var MetricKind_value = map[string]int32{
	"INVALID_METRIC_KIND": 0,
	"COUNTER":             1,
	"GAUGE":               2,
}

func (x MetricKind) String() string {
	return proto.EnumName(MetricKind_name, int32(x))
}
func (MetricKind) EnumDescriptor() ([]byte, []int) { return fileDescriptorMetrics, []int{0} }

// MetricPoint is an update to a single measure.
type MetricPoint struct {
	// Kind indicates the semantics of this point. Kind should always be the same for a given metric
	// name (e.g. "cpu.usage" should always have the same kind)
	Kind MetricKind `protobuf:"varint,1,opt,name=kind,proto3,enum=lightstep.metrics.MetricKind" json:"kind,omitempty"`
	// MetricName indicates the metric being emitted.
	MetricName string `protobuf:"bytes,2,opt,name=metric_name,json=metricName,proto3" json:"metric_name,omitempty"`
	// Start of the interval for which the points represent.
	// - All Counter points will be assumed to represent the entire interval.
	// - All Gauge points will be assumed to be instantaneous at the start of the interval.
	Start *google_protobuf.Timestamp `protobuf:"bytes,3,opt,name=start" json:"start,omitempty"`
	// Duration of the interval for which the points represent. The end of the interval is start + duration.
	// We expect this value to be unset or zero for Gauge points.
	Duration *google_protobuf2.Duration `protobuf:"bytes,4,opt,name=duration" json:"duration,omitempty"`
	// Labels contain labels specific to this point.
	Labels []*lightstep_collector.KeyValue `protobuf:"bytes,5,rep,name=labels" json:"labels,omitempty"`
	// Value represents the update being emitted. Values can be one of two types: uint64 or double.
	// The type of the value should always be the same for a given metric name (e.g. "cpu.usage"
	// should always have value type double).
	//
	// Types that are valid to be assigned to Value:
	//	*MetricPoint_Uint64Value
	//	*MetricPoint_DoubleValue
	Value isMetricPoint_Value `protobuf_oneof:"value"`
}

func (m *MetricPoint) Reset()                    { *m = MetricPoint{} }
func (m *MetricPoint) String() string            { return proto.CompactTextString(m) }
func (*MetricPoint) ProtoMessage()               {}
func (*MetricPoint) Descriptor() ([]byte, []int) { return fileDescriptorMetrics, []int{0} }

type isMetricPoint_Value interface {
	isMetricPoint_Value()
	MarshalTo([]byte) (int, error)
	Size() int
}

type MetricPoint_Uint64Value struct {
	Uint64Value uint64 `protobuf:"varint,6,opt,name=uint64_value,json=uint64Value,proto3,oneof"`
}
type MetricPoint_DoubleValue struct {
	DoubleValue float64 `protobuf:"fixed64,7,opt,name=double_value,json=doubleValue,proto3,oneof"`
}

func (*MetricPoint_Uint64Value) isMetricPoint_Value() {}
func (*MetricPoint_DoubleValue) isMetricPoint_Value() {}

func (m *MetricPoint) GetValue() isMetricPoint_Value {
	if m != nil {
		return m.Value
	}
	return nil
}

func (m *MetricPoint) GetKind() MetricKind {
	if m != nil {
		return m.Kind
	}
	return MetricKind_INVALID_METRIC_KIND
}

func (m *MetricPoint) GetMetricName() string {
	if m != nil {
		return m.MetricName
	}
	return ""
}

func (m *MetricPoint) GetStart() *google_protobuf.Timestamp {
	if m != nil {
		return m.Start
	}
	return nil
}

func (m *MetricPoint) GetDuration() *google_protobuf2.Duration {
	if m != nil {
		return m.Duration
	}
	return nil
}

func (m *MetricPoint) GetLabels() []*lightstep_collector.KeyValue {
	if m != nil {
		return m.Labels
	}
	return nil
}

func (m *MetricPoint) GetUint64Value() uint64 {
	if x, ok := m.GetValue().(*MetricPoint_Uint64Value); ok {
		return x.Uint64Value
	}
	return 0
}

func (m *MetricPoint) GetDoubleValue() float64 {
	if x, ok := m.GetValue().(*MetricPoint_DoubleValue); ok {
		return x.DoubleValue
	}
	return 0
}

// XXX_OneofFuncs is for the internal use of the proto package.
func (*MetricPoint) XXX_OneofFuncs() (func(msg proto.Message, b *proto.Buffer) error, func(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error), func(msg proto.Message) (n int), []interface{}) {
	return _MetricPoint_OneofMarshaler, _MetricPoint_OneofUnmarshaler, _MetricPoint_OneofSizer, []interface{}{
		(*MetricPoint_Uint64Value)(nil),
		(*MetricPoint_DoubleValue)(nil),
	}
}

func _MetricPoint_OneofMarshaler(msg proto.Message, b *proto.Buffer) error {
	m := msg.(*MetricPoint)
	// value
	switch x := m.Value.(type) {
	case *MetricPoint_Uint64Value:
		_ = b.EncodeVarint(6<<3 | proto.WireVarint)
		_ = b.EncodeVarint(uint64(x.Uint64Value))
	case *MetricPoint_DoubleValue:
		_ = b.EncodeVarint(7<<3 | proto.WireFixed64)
		_ = b.EncodeFixed64(math.Float64bits(x.DoubleValue))
	case nil:
	default:
		return fmt.Errorf("MetricPoint.Value has unexpected type %T", x)
	}
	return nil
}

func _MetricPoint_OneofUnmarshaler(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error) {
	m := msg.(*MetricPoint)
	switch tag {
	case 6: // value.uint64_value
		if wire != proto.WireVarint {
			return true, proto.ErrInternalBadWireType
		}
		x, err := b.DecodeVarint()
		m.Value = &MetricPoint_Uint64Value{x}
		return true, err
	case 7: // value.double_value
		if wire != proto.WireFixed64 {
			return true, proto.ErrInternalBadWireType
		}
		x, err := b.DecodeFixed64()
		m.Value = &MetricPoint_DoubleValue{math.Float64frombits(x)}
		return true, err
	default:
		return false, nil
	}
}

func _MetricPoint_OneofSizer(msg proto.Message) (n int) {
	m := msg.(*MetricPoint)
	// value
	switch x := m.Value.(type) {
	case *MetricPoint_Uint64Value:
		n += proto.SizeVarint(6<<3 | proto.WireVarint)
		n += proto.SizeVarint(uint64(x.Uint64Value))
	case *MetricPoint_DoubleValue:
		n += proto.SizeVarint(7<<3 | proto.WireFixed64)
		n += 8
	case nil:
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in oneof", x))
	}
	return n
}

// IngestRequest is an update to one or more measures.
type IngestRequest struct {
	// IdempotencyKey is a random string that should uniquely identify this report.
	// It should be generated once and used for all retries. The server will use it
	// to de-duplicate requests.
	IdempotencyKey string `protobuf:"bytes,1,opt,name=idempotency_key,json=idempotencyKey,proto3" json:"idempotency_key,omitempty"`
	// Reporter contains information to identify the specific originator of this report.
	Reporter *lightstep_collector.Reporter `protobuf:"bytes,2,opt,name=reporter" json:"reporter,omitempty"`
	// Points contain the individual updates.
	Points []*MetricPoint `protobuf:"bytes,3,rep,name=points" json:"points,omitempty"`
}

func (m *IngestRequest) Reset()                    { *m = IngestRequest{} }
func (m *IngestRequest) String() string            { return proto.CompactTextString(m) }
func (*IngestRequest) ProtoMessage()               {}
func (*IngestRequest) Descriptor() ([]byte, []int) { return fileDescriptorMetrics, []int{1} }

func (m *IngestRequest) GetIdempotencyKey() string {
	if m != nil {
		return m.IdempotencyKey
	}
	return ""
}

func (m *IngestRequest) GetReporter() *lightstep_collector.Reporter {
	if m != nil {
		return m.Reporter
	}
	return nil
}

func (m *IngestRequest) GetPoints() []*MetricPoint {
	if m != nil {
		return m.Points
	}
	return nil
}

// IngestResponse is reserved for future use.
type IngestResponse struct {
}

func (m *IngestResponse) Reset()                    { *m = IngestResponse{} }
func (m *IngestResponse) String() string            { return proto.CompactTextString(m) }
func (*IngestResponse) ProtoMessage()               {}
func (*IngestResponse) Descriptor() ([]byte, []int) { return fileDescriptorMetrics, []int{2} }

func init() {
	proto.RegisterType((*MetricPoint)(nil), "lightstep.metrics.MetricPoint")
	proto.RegisterType((*IngestRequest)(nil), "lightstep.metrics.IngestRequest")
	proto.RegisterType((*IngestResponse)(nil), "lightstep.metrics.IngestResponse")
	proto.RegisterEnum("lightstep.metrics.MetricKind", MetricKind_name, MetricKind_value)
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// Client API for MetricsService service

type MetricsServiceClient interface {
	Report(ctx context.Context, in *IngestRequest, opts ...grpc.CallOption) (*IngestResponse, error)
}

type metricsServiceClient struct {
	cc *grpc.ClientConn
}

func NewMetricsServiceClient(cc *grpc.ClientConn) MetricsServiceClient {
	return &metricsServiceClient{cc}
}

func (c *metricsServiceClient) Report(ctx context.Context, in *IngestRequest, opts ...grpc.CallOption) (*IngestResponse, error) {
	out := new(IngestResponse)
	err := grpc.Invoke(ctx, "/lightstep.metrics.MetricsService/Report", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for MetricsService service

type MetricsServiceServer interface {
	Report(context.Context, *IngestRequest) (*IngestResponse, error)
}

func RegisterMetricsServiceServer(s *grpc.Server, srv MetricsServiceServer) {
	s.RegisterService(&_MetricsService_serviceDesc, srv)
}

func _MetricsService_Report_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(IngestRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MetricsServiceServer).Report(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/lightstep.metrics.MetricsService/Report",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MetricsServiceServer).Report(ctx, req.(*IngestRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _MetricsService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "lightstep.metrics.MetricsService",
	HandlerType: (*MetricsServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Report",
			Handler:    _MetricsService_Report_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "github.com/lightstep/lightstep-tracer-common/tmpgen/metrics.proto",
}

func (m *MetricPoint) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *MetricPoint) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.Kind != 0 {
		dAtA[i] = 0x8
		i++
		i = encodeVarintMetrics(dAtA, i, uint64(m.Kind))
	}
	if len(m.MetricName) > 0 {
		dAtA[i] = 0x12
		i++
		i = encodeVarintMetrics(dAtA, i, uint64(len(m.MetricName)))
		i += copy(dAtA[i:], m.MetricName)
	}
	if m.Start != nil {
		dAtA[i] = 0x1a
		i++
		i = encodeVarintMetrics(dAtA, i, uint64(m.Start.Size()))
		n1, err := m.Start.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n1
	}
	if m.Duration != nil {
		dAtA[i] = 0x22
		i++
		i = encodeVarintMetrics(dAtA, i, uint64(m.Duration.Size()))
		n2, err := m.Duration.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n2
	}
	if len(m.Labels) > 0 {
		for _, msg := range m.Labels {
			dAtA[i] = 0x2a
			i++
			i = encodeVarintMetrics(dAtA, i, uint64(msg.Size()))
			n, err := msg.MarshalTo(dAtA[i:])
			if err != nil {
				return 0, err
			}
			i += n
		}
	}
	if m.Value != nil {
		nn3, err := m.Value.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += nn3
	}
	return i, nil
}

func (m *MetricPoint_Uint64Value) MarshalTo(dAtA []byte) (int, error) {
	i := 0
	dAtA[i] = 0x30
	i++
	i = encodeVarintMetrics(dAtA, i, uint64(m.Uint64Value))
	return i, nil
}
func (m *MetricPoint_DoubleValue) MarshalTo(dAtA []byte) (int, error) {
	i := 0
	dAtA[i] = 0x39
	i++
	encoding_binary.LittleEndian.PutUint64(dAtA[i:], uint64(math.Float64bits(float64(m.DoubleValue))))
	i += 8
	return i, nil
}
func (m *IngestRequest) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *IngestRequest) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if len(m.IdempotencyKey) > 0 {
		dAtA[i] = 0xa
		i++
		i = encodeVarintMetrics(dAtA, i, uint64(len(m.IdempotencyKey)))
		i += copy(dAtA[i:], m.IdempotencyKey)
	}
	if m.Reporter != nil {
		dAtA[i] = 0x12
		i++
		i = encodeVarintMetrics(dAtA, i, uint64(m.Reporter.Size()))
		n4, err := m.Reporter.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n4
	}
	if len(m.Points) > 0 {
		for _, msg := range m.Points {
			dAtA[i] = 0x1a
			i++
			i = encodeVarintMetrics(dAtA, i, uint64(msg.Size()))
			n, err := msg.MarshalTo(dAtA[i:])
			if err != nil {
				return 0, err
			}
			i += n
		}
	}
	return i, nil
}

func (m *IngestResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *IngestResponse) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	return i, nil
}

func encodeVarintMetrics(dAtA []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return offset + 1
}
func (m *MetricPoint) Size() (n int) {
	var l int
	_ = l
	if m.Kind != 0 {
		n += 1 + sovMetrics(uint64(m.Kind))
	}
	l = len(m.MetricName)
	if l > 0 {
		n += 1 + l + sovMetrics(uint64(l))
	}
	if m.Start != nil {
		l = m.Start.Size()
		n += 1 + l + sovMetrics(uint64(l))
	}
	if m.Duration != nil {
		l = m.Duration.Size()
		n += 1 + l + sovMetrics(uint64(l))
	}
	if len(m.Labels) > 0 {
		for _, e := range m.Labels {
			l = e.Size()
			n += 1 + l + sovMetrics(uint64(l))
		}
	}
	if m.Value != nil {
		n += m.Value.Size()
	}
	return n
}

func (m *MetricPoint_Uint64Value) Size() (n int) {
	var l int
	_ = l
	n += 1 + sovMetrics(uint64(m.Uint64Value))
	return n
}
func (m *MetricPoint_DoubleValue) Size() (n int) {
	var l int
	_ = l
	n += 9
	return n
}
func (m *IngestRequest) Size() (n int) {
	var l int
	_ = l
	l = len(m.IdempotencyKey)
	if l > 0 {
		n += 1 + l + sovMetrics(uint64(l))
	}
	if m.Reporter != nil {
		l = m.Reporter.Size()
		n += 1 + l + sovMetrics(uint64(l))
	}
	if len(m.Points) > 0 {
		for _, e := range m.Points {
			l = e.Size()
			n += 1 + l + sovMetrics(uint64(l))
		}
	}
	return n
}

func (m *IngestResponse) Size() (n int) {
	var l int
	_ = l
	return n
}

func sovMetrics(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozMetrics(x uint64) (n int) {
	return sovMetrics(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *MetricPoint) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowMetrics
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: MetricPoint: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: MetricPoint: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Kind", wireType)
			}
			m.Kind = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMetrics
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Kind |= (MetricKind(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field MetricName", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMetrics
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthMetrics
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.MetricName = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Start", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMetrics
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthMetrics
			}
			postIndex := iNdEx + msglen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Start == nil {
				m.Start = &google_protobuf.Timestamp{}
			}
			if err := m.Start.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Duration", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMetrics
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthMetrics
			}
			postIndex := iNdEx + msglen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Duration == nil {
				m.Duration = &google_protobuf2.Duration{}
			}
			if err := m.Duration.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 5:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Labels", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMetrics
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthMetrics
			}
			postIndex := iNdEx + msglen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Labels = append(m.Labels, &lightstep_collector.KeyValue{})
			if err := m.Labels[len(m.Labels)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 6:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Uint64Value", wireType)
			}
			var v uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMetrics
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.Value = &MetricPoint_Uint64Value{v}
		case 7:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field DoubleValue", wireType)
			}
			var v uint64
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			v = uint64(encoding_binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
			m.Value = &MetricPoint_DoubleValue{float64(math.Float64frombits(v))}
		default:
			iNdEx = preIndex
			skippy, err := skipMetrics(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthMetrics
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *IngestRequest) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowMetrics
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: IngestRequest: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: IngestRequest: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field IdempotencyKey", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMetrics
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthMetrics
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.IdempotencyKey = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Reporter", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMetrics
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthMetrics
			}
			postIndex := iNdEx + msglen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Reporter == nil {
				m.Reporter = &lightstep_collector.Reporter{}
			}
			if err := m.Reporter.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Points", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMetrics
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthMetrics
			}
			postIndex := iNdEx + msglen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Points = append(m.Points, &MetricPoint{})
			if err := m.Points[len(m.Points)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipMetrics(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthMetrics
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *IngestResponse) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowMetrics
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: IngestResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: IngestResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		default:
			iNdEx = preIndex
			skippy, err := skipMetrics(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthMetrics
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipMetrics(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowMetrics
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowMetrics
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
			return iNdEx, nil
		case 1:
			iNdEx += 8
			return iNdEx, nil
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowMetrics
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			iNdEx += length
			if length < 0 {
				return 0, ErrInvalidLengthMetrics
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowMetrics
					}
					if iNdEx >= l {
						return 0, io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					innerWire |= (uint64(b) & 0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				innerWireType := int(innerWire & 0x7)
				if innerWireType == 4 {
					break
				}
				next, err := skipMetrics(dAtA[start:])
				if err != nil {
					return 0, err
				}
				iNdEx = start + next
			}
			return iNdEx, nil
		case 4:
			return iNdEx, nil
		case 5:
			iNdEx += 4
			return iNdEx, nil
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
	}
	panic("unreachable")
}

var (
	ErrInvalidLengthMetrics = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowMetrics   = fmt.Errorf("proto: integer overflow")
)

func init() {
	proto.RegisterFile("github.com/lightstep/lightstep-tracer-common/tmpgen/metrics.proto", fileDescriptorMetrics)
}

var fileDescriptorMetrics = []byte{
	// 603 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x94, 0x93, 0xcf, 0x6e, 0xd3, 0x4c,
	0x14, 0xc5, 0x3b, 0x69, 0x92, 0x36, 0x93, 0xaf, 0xf9, 0xc2, 0x74, 0x81, 0x1b, 0x41, 0x6a, 0xc2,
	0x02, 0xab, 0x52, 0x6d, 0x28, 0xb4, 0x12, 0x48, 0x08, 0xf5, 0x9f, 0x4a, 0x94, 0x36, 0x54, 0xd3,
	0xb4, 0x0b, 0x36, 0x91, 0xed, 0x5c, 0x5c, 0xab, 0xf6, 0x8c, 0x19, 0x8f, 0x2b, 0x65, 0xcb, 0x13,
	0x20, 0xf1, 0x06, 0x6c, 0x79, 0x11, 0x96, 0x48, 0xf0, 0x00, 0xa8, 0xf0, 0x20, 0x28, 0x33, 0x4e,
	0xd2, 0x52, 0x8a, 0xc4, 0x6e, 0x72, 0xcf, 0xef, 0x4e, 0xce, 0xb9, 0xd7, 0x83, 0x37, 0x83, 0x50,
	0x9e, 0x66, 0x9e, 0xed, 0xf3, 0xd8, 0x89, 0xc2, 0xe0, 0x54, 0xa6, 0x12, 0x92, 0xe9, 0x69, 0x55,
	0x0a, 0xd7, 0x07, 0xb1, 0xea, 0xf3, 0x38, 0xe6, 0xcc, 0x91, 0x71, 0x12, 0x00, 0x73, 0x62, 0x90,
	0x22, 0xf4, 0x53, 0x3b, 0x11, 0x5c, 0x72, 0x72, 0x6b, 0x42, 0xdb, 0xb9, 0xd0, 0xe8, 0xfd, 0xd3,
	0xad, 0x01, 0x8f, 0x5c, 0x16, 0x38, 0x01, 0x0f, 0xb8, 0xe3, 0xf3, 0x28, 0x02, 0x5f, 0x72, 0x91,
	0x78, 0xd3, 0xb3, 0xfe, 0xa3, 0xc6, 0x9d, 0x80, 0xf3, 0x20, 0x02, 0xc7, 0x4d, 0x42, 0xc7, 0x65,
	0x8c, 0x4b, 0x57, 0x86, 0x9c, 0xe5, 0x36, 0x1a, 0xcd, 0x5c, 0x55, 0xbf, 0xbc, 0xec, 0x8d, 0x33,
	0xc8, 0x84, 0x02, 0x72, 0x7d, 0xf9, 0x77, 0x5d, 0x86, 0x31, 0xa4, 0xd2, 0x8d, 0x13, 0x0d, 0xb4,
	0xbe, 0x15, 0x70, 0xf5, 0x40, 0x05, 0x38, 0xe4, 0x21, 0x93, 0xe4, 0x11, 0x2e, 0x9e, 0x85, 0x6c,
	0x60, 0x20, 0x13, 0x59, 0xb5, 0xb5, 0xbb, 0xf6, 0xb5, 0x98, 0xb6, 0xa6, 0x3b, 0x21, 0x1b, 0x50,
	0x85, 0x92, 0x65, 0x5c, 0xd5, 0x5a, 0x9f, 0xb9, 0x31, 0x18, 0x05, 0x13, 0x59, 0x15, 0x8a, 0x75,
	0xa9, 0xeb, 0xc6, 0x40, 0x1e, 0xe2, 0x52, 0x2a, 0x5d, 0x21, 0x8d, 0x59, 0x13, 0x59, 0xd5, 0xb5,
	0x86, 0xad, 0x4d, 0xd9, 0x63, 0x53, 0x76, 0x6f, 0x6c, 0x8a, 0x6a, 0x90, 0xac, 0xe3, 0xf9, 0x71,
	0x10, 0xa3, 0xa8, 0x9a, 0x96, 0xae, 0x35, 0xed, 0xe4, 0x00, 0x9d, 0xa0, 0x64, 0x1d, 0x97, 0x23,
	0xd7, 0x83, 0x28, 0x35, 0x4a, 0xe6, 0xac, 0x55, 0xbd, 0x62, 0x7f, 0x3a, 0xd7, 0x0e, 0x0c, 0x4f,
	0xdc, 0x28, 0x03, 0x9a, 0xc3, 0xe4, 0x3e, 0xfe, 0x2f, 0x0b, 0x99, 0xdc, 0x78, 0xd2, 0x3f, 0x1f,
	0xd5, 0x8d, 0xb2, 0x89, 0xac, 0xe2, 0xcb, 0x19, 0x5a, 0xd5, 0x55, 0x05, 0x8f, 0xa0, 0x01, 0xcf,
	0xbc, 0x08, 0x72, 0x68, 0xce, 0x44, 0x16, 0x1a, 0x41, 0xba, 0xaa, 0xa0, 0xad, 0x39, 0x5c, 0x52,
	0x6a, 0xeb, 0x13, 0xc2, 0x0b, 0x6d, 0x16, 0x40, 0x2a, 0x29, 0xbc, 0xcd, 0x20, 0x95, 0xe4, 0x01,
	0xfe, 0x3f, 0x1c, 0x40, 0x9c, 0x70, 0x09, 0xcc, 0x1f, 0xf6, 0xcf, 0x60, 0xa8, 0x66, 0x5c, 0xa1,
	0xb5, 0x4b, 0xe5, 0x0e, 0x0c, 0xc9, 0x53, 0x3c, 0x2f, 0x20, 0xe1, 0x42, 0x82, 0x50, 0xb3, 0xbc,
	0x29, 0x06, 0xcd, 0x21, 0x3a, 0xc1, 0xc9, 0x06, 0x2e, 0x27, 0xa3, 0x2d, 0xa6, 0xc6, 0xac, 0xca,
	0xdf, 0xbc, 0x71, 0x7d, 0x6a, 0xd9, 0x34, 0xa7, 0x5b, 0x75, 0x5c, 0x1b, 0x9b, 0x4d, 0x13, 0xce,
	0x52, 0x58, 0x79, 0x8e, 0xf1, 0x74, 0xcf, 0xe4, 0x36, 0x5e, 0x6c, 0x77, 0x4f, 0x36, 0xf7, 0xdb,
	0x3b, 0xfd, 0x83, 0xdd, 0x1e, 0x6d, 0x6f, 0xf7, 0x3b, 0xed, 0xee, 0x4e, 0x7d, 0x86, 0x54, 0xf1,
	0xdc, 0xf6, 0xab, 0xe3, 0x6e, 0x6f, 0x97, 0xd6, 0x11, 0xa9, 0xe0, 0xd2, 0xde, 0xe6, 0xf1, 0xde,
	0x6e, 0xbd, 0xb0, 0x26, 0x71, 0x4d, 0xb7, 0xa7, 0x47, 0x20, 0xce, 0x43, 0x1f, 0x88, 0x87, 0xcb,
	0xda, 0x30, 0x31, 0xff, 0x60, 0xea, 0xca, 0xa8, 0x1a, 0xf7, 0xfe, 0x42, 0x68, 0x7f, 0xad, 0xc5,
	0x77, 0x5f, 0x7f, 0x7e, 0x28, 0x2c, 0xb4, 0xe6, 0xc7, 0xcf, 0xf2, 0x19, 0x5a, 0xd9, 0x7a, 0xf1,
	0xf9, 0xa2, 0x89, 0xbe, 0x5c, 0x34, 0xd1, 0xf7, 0x8b, 0x26, 0x7a, 0xff, 0xa3, 0x39, 0x83, 0x97,
	0x7c, 0x1e, 0x5f, 0xba, 0x4c, 0xbf, 0x40, 0x3b, 0x10, 0x89, 0x7f, 0x88, 0x5e, 0x57, 0xf2, 0xd6,
	0xc4, 0xfb, 0x58, 0x28, 0xee, 0x1f, 0x1d, 0x6e, 0x79, 0x65, 0xf5, 0x71, 0x3d, 0xfe, 0x15, 0x00,
	0x00, 0xff, 0xff, 0xbe, 0x98, 0xa2, 0xa5, 0x20, 0x04, 0x00, 0x00,
}