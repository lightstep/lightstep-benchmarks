import generated.collector_pb2 as collector
import requests

report_request = collector.ReportRequest()

span = collector.Span()
span.operation_name = "isaac_op"
report_request.spans.append(span)

binary_report_request = report_request.SerializeToString()

res = requests.post(url='http://localhost:8012',
                    data=binary_report_request,
                    headers={'Content-Type': 'application/octet-stream'})
