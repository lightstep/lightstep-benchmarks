const lightstep = require("lightstep-tracer");
const minimist = require("minimist");
/*
  // args come in as json object of the following format
  {"_":[],"trace":1,"sleep":15076976,"sleep_interval":100000000,"work":3593930,"repeat":1000,"no_flush":0,"num_tags":10,"num_logs":15}
*/
const args = minimist(process.argv.slice(2));
let maybeTransport = minimist(process.argv.slice(2))._[0];
let transport = "thrift";
if (maybeTransport && maybeTransport == "proto") {
  transport = "proto";
}
// test args
// const args = {"_":[],"trace":1,"sleep":15076976,"sleep_interval":100000000,"work":3593930,"repeat":1000,"no_flush":0,"num_tags":10,"num_logs":15};

/* The only args this client uses/supports right now are:
  num_tags,
  num_logs,
  work,
  repeat,
  no_flush

  the args this client does not yet support are
  trace,
  sleep,
  sleep_interval

  To support sleep/sleep interval, would need to have some additional wait (set timeout) in the generate span function after depth is at 0
  Also see how this is done in cpp/python client. */

const SPANS_PER_LOOP = 6, // depth of each trace
  SATELLITE_PORTS = [8360, 8361, 8362, 8363, 8364, 8365, 8366, 8367];

/*
These settings below would be interesting to support in the future

To do so, you would disable the reporting loop in tracer options
Option is: disable_reporting_loop

and then manually create a new reporting loop based on both of these
(REPORTING_PERIOD is the reporting interval), MAX_BUFFERED_SPANS is the max to buffer before dropping
*/

const MAX_BUFFERED_SPANS = 10000,
  REPORTING_PERIOD = 0.2;

const tags = [],
  logs = [];

// set up logs and tags based on incoming args for num_tags and num_logs
const setupTagsAndLogs = () => {
  let i = 0;

  for (i of Array(args.num_tags).keys()) {
    tags.push([`tag.key${i}`, `tag.value${i}`]);
  }
  for (i of Array(args.num_logs).keys()) {
    logs.push([`log.key${i}`, `log.value${i}`]);
  }
};

// do some floating point calculations to represent work
const doWork = units => {
  return new Promise(resolve => {
    let i = 1.12563,
      j = 0;
    for (j of Array(units).keys()) {
      i *= i;
    }
    resolve();
  });
};

// generate span (chained promise) of given depth
const generateSpan = (tracer, depth, units, parent = null) => {
  let nspan = parent
    ? tracer.startSpan("child", { childOf: parent.context() })
    : tracer.startSpan("parent");
  tags.forEach(t => nspan.setTag(t[0], t[1]));
  logs.forEach(l => {
    let log = {};
    log[l[0]] = l[1];
    nspan.log(log);
  });
  depth -= 1;

  if (depth > 0) {
    return doWork(units)
      .then(() => generateSpan(tracer, depth, units, nspan))
      .then(() => nspan.finish());
  } else {
    // add setTimeout here for sleep/sleep interval support
    return doWork(units).then(() => nspan.finish());
  }
};

// build tracer based on args and return
const buildTracer = () => {
  const tracer = new lightstep.Tracer({
    transport: transport, // IMPORTANT: THIS IS WHERE WE CAN CHANGE TO "PROTO" AND SEE TRANSPORT PERF DIFFERENCE
    component_name: "test",
    disable_report_on_exit: Boolean(args.no_flush),
    // disable_reporting_loop: false, // would need to set this to true and call flush manually for our own reporting period
    // access_token: '', // for testing on public
    // comment out below options if using public
    access_token: "developer",
    collector_port: SATELLITE_PORTS[0],
    collector_host: "localhost",
    collector_encryption: "none",
    verbosity: 4
  });
  return tracer;
};

const generateTraces = (unitsWork, depth, repeat) => {
  if (repeat < 1) {
    throw new Error("Number of traces to send (repeat variable) must be > 0.");
  }

  const tracer = buildTracer();
  const work = [];
  let i = 0;

  for (i of Array(repeat).keys()) {
    work.push(generateSpan(tracer, depth, unitsWork));
  }
  return Promise.all(work);
};

setupTagsAndLogs();
generateTraces(args.work, SPANS_PER_LOOP, args.repeat);
