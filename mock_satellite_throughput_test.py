import generated.collector_pb2 as collector
import requests
from time import time, sleep
from satellite_wrapper import MockSatelliteGroup

SPANS_IN_REPORT_REQUEST = 100
TEST_LENGTH = 10

# make a very simple 50 span report
report_request = collector.ReportRequest()
span = collector.Span()
span.operation_name = "isaac_op"
for i in range(SPANS_IN_REPORT_REQUEST):
    report_request.spans.append(span)
binary_report_request = report_request.SerializeToString()

# startup the mock satellite
mock_satellite = MockSatelliteGroup([8012])
sleep(1)

try:
    start_time = time()
    spans_sent = 0
    while time() < start_time + TEST_LENGTH:
        res = requests.post(url='http://localhost:8012/api/v2/reports',
                            data=binary_report_request,
                            headers={'Content-Type': 'application/octet-stream'})
        spans_sent += SPANS_IN_REPORT_REQUEST

    test_time = time() - start_time
    spans_received = mock_satellite.get_spans_received()

    print(f'dropped {(spans_sent - spans_received)} spans')
    print(f'average rate of {(spans_sent / test_time):.1f} spans / second')

finally:
    mock_satellite.terminate()
