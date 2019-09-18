#include <algorithm>
#include <cmath>
#include <thread>
#include <vector>
#include <string>
#include <utility>

#include <gflags/gflags.h>
#include <lightstep/tracer.h>
#include <opentracing/noop.h>
std::initializer_list<uint16_t> SatellitePorts = {8360, 8361, 8362, 8363,
                                                  8364, 8365, 8366, 8367};
const size_t MaxBufferedSpans = 10000;
const int SpansPerLoop = 6;
const std::chrono::steady_clock::duration ReportingPeriod =
    std::chrono::duration_cast<std::chrono::steady_clock::duration>(
        std::chrono::milliseconds{200});

DEFINE_string(tracer, "", "Which LightStep tracer to use");
DEFINE_int32(trace, 0, "Whether to trace");
DEFINE_double(sleep, 0.0, "The amount of time to sleep for each span");
DEFINE_int32(sleep_interval, 0, "The duration of each sleep");
DEFINE_int32(work, 0, "The quantity of work to perform between spans");
DEFINE_int32(repeat, 0, "The number of span generation repetitions to perform");
DEFINE_int32(no_flush, 0, "Whether to flush on finishing");
DEFINE_int32(num_tags, 0, "The number of tags to annotate spans with");
DEFINE_int32(num_logs, 0, "The number of logs to annotate spans with");

std::vector<std::pair<std::string, std::string>> Tags;
std::vector<std::pair<std::string, std::string>> Logs;

static void setup_annotations() {
  Tags.reserve(FLAGS_num_tags);
  for (int i = 0; i < FLAGS_num_tags; ++i) {
    Tags.emplace_back("tag.key" + std::to_string(i), "tag.value" + std::to_string(i));
  }
  Logs.reserve(FLAGS_num_logs);
  for (int i = 0; i < FLAGS_num_logs; ++i) {
    Logs.emplace_back("log.key" + std::to_string(i), "log.value" + std::to_string(i));
  }
}

template <class T>
static void do_not_optimize_away(T&& x) {
  asm volatile("" : "+r"(x));
}

static void do_work(int quantity) {
  double x = 1.12563;
  for (int i = 0; i < quantity; ++i) {
    x *= std::sqrt(std::log(static_cast<double>(i + 5)));
  }
  do_not_optimize_away(x);
}

static std::unique_ptr<opentracing::Span> make_span(opentracing::Tracer& tracer,
    const opentracing::SpanContext* parent_context) {
  auto span = tracer.StartSpan("isaac_service", {opentracing::ChildOf(parent_context)});
  for (auto& tag : Tags) {
    span->SetTag(tag.first, tag.second.data());
  }
  for (auto& log : Logs) {
    span->Log({{log.first, log.second.data()}});
  }
  return span;
}

static void generate_spans(opentracing::Tracer& tracer, int work_quantity,
                           int num_spans,
                           const opentracing::SpanContext* parent_context) {
  auto client_span = make_span(tracer, parent_context);
  do_work(work_quantity);
  num_spans -= 1;
  if (num_spans == 0) {
    return;
  }

  auto server_span = make_span(tracer, &client_span->context());
  do_work(work_quantity);
  num_spans -= 1;
  if (num_spans == 0) {
    return;
  }

  auto db_span = make_span(tracer, &server_span->context());
  do_work(work_quantity);
  num_spans -= 1;
  if (num_spans == 0) {
    return;
  }

  generate_spans(tracer, work_quantity, num_spans, &db_span->context());
}

static std::shared_ptr<opentracing::Tracer> build_tracer() {
  if (FLAGS_trace == 0) {
    return opentracing::MakeNoopTracer();
  }
  lightstep::LightStepTracerOptions options;
  options.component_name = "isaac_service";
  options.access_token = "developer";
  options.collector_plaintext = true;
  options.use_stream_recorder = true;
  options.max_buffered_spans = MaxBufferedSpans;
  options.reporting_period = ReportingPeriod;
  options.satellite_endpoints.clear();
  options.satellite_endpoints.reserve(SatellitePorts.size());
  for (auto port : SatellitePorts) {
    options.satellite_endpoints.emplace_back("127.0.0.1", port);
  }
  return lightstep::MakeLightStepTracer(std::move(options));
}

static void perform_work() {
  auto tracer = build_tracer();

  double sleep_debt = 0.0;
  int spans_sent = 0;
  while (spans_sent < FLAGS_repeat) {
    auto spans_to_send = std::min(FLAGS_repeat - spans_sent, SpansPerLoop);
    generate_spans(*tracer, FLAGS_work, spans_to_send, nullptr);
    spans_sent += spans_to_send;
    sleep_debt += FLAGS_sleep * spans_to_send;
    if (sleep_debt > FLAGS_sleep_interval) {
      sleep_debt -= FLAGS_sleep_interval;
      std::this_thread::sleep_for(
          std::chrono::nanoseconds{FLAGS_sleep_interval});
    }
  }
  if (FLAGS_no_flush != 1) {
    tracer->Close();
  }
}

int main(int argc, char* argv[]) {
  gflags::ParseCommandLineFlags(&argc, &argv, true);
  setup_annotations();
  perform_work();
  return 0;
}
