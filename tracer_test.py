import lightstep
import opentracing



opentracing.tracer = lightstep.Tracer(
component_name='isaac_service',
collector_port=8012,
collector_host='localhost',
collector_encryption='none',
use_http=True,
access_token='developer'
)

with opentracing.tracer.start_active_span('TestSpan') as scope:
    pass

opentracing.tracer.flush()
