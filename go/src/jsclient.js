'use strict';

// note: this test client exhibits strange performance that is
// non-deterministic.
//
// first, occasionally a run of measurespanthroughput using the no-op
// tracer takes significantly longer than expected (after
// calibration), with the divergence increasing as the load increases.
// this could be reproduced, (non-deterministically), with the debug
// and non-debug versions of the ls library. (e.g., an untraced test 
// that should run 10 seconds runs 15 seconds). is this a scheduling
// problem?
//
// second, when turning on nodejs profiling support, the
// measurespanthroughput test runs significantly faster than expected
// (after calibration), with the divergence increasing as the load
// increases. (e.g., an untraced test that should run 30 seconds runs
// 15 seconds).

const opentracing = require('opentracing');
const lightstep   = require('lightstep-tracer');
const http        = require('http');

const host = 'localhost';
const port = 8000;

const millis_per_nano = 1000000;
const prime_work = 982451653;

const noop_tracer = opentracing.initNewTracer(null);
const test_tracer = opentracing.initNewTracer(lightstep.tracer({
    access_token         : 'ignored',
    collector_port       : port,
    collector_host       : host,
    collector_encryption : 'none',
    component_name       : 'javascript/test',
}));

var log_input_string = "";

for (var i = 0; i < 1<<20; i++) {
    log_input_string += String.fromCharCode(65 + (i%26));
}

// Note: Keep-Alive does not work properly, reason unknown.  Disable
// it to make progress.
var keepAliveAgent = new http.Agent({
    keepAlive: false,
});

function get_control(response) {
    var body = '';
    response.on('data', function(d) {
        body += d;
	return;
    });
    response.on('end', function() {
	var c = JSON.parse(body)
	if (c.Exit) {
	    process.exit();
	}
	var tracer = c.Trace ? test_tracer : noop_tracer;
	return exec_control(c, tracer);
    });
}

function next_control() {
    process.nextTick(function() {
	return http.get({
	    host: host,
	    port: port,
	    path: '/control',
	    agent: keepAliveAgent,
	}, get_control);
    }, 0);
}

function exec_control(c, tracer) {
    // Force garbage collection (requires the --enable-gc flag)
    // global.gc()

    var begin = process.hrtime();
    var sleep_debt = 0;
    var sleep_at;
    var sleep_nanos = '';
    var p = prime_work;
    var body_func = function(repeat) {
	if (sleep_debt > 0) {
	    var diff = process.hrtime(sleep_at);
	    var nanos = diff[0] * 1e9 + diff[1];
	    sleep_nanos = sleep_nanos + nanos + ','
	    sleep_debt -= nanos
	}
	for (var r = repeat; r > 0; r--) {
	    var span = tracer.startSpan("span/test");
	    for (var i = 0; i < c.Work; i++) {
		p *= p
		p %= 2147483647
	    }
	    for (var i = 0; i < c.NumLogs; i++) {
		span.logEvent("testlog", log_input_string.substr(0, c.BytesPerLog));
	    }
	    span.finish()
	    // Note: We go through this code section even when c.Sleep == 0
	    // for measurement consistency.
	    sleep_debt += c.Sleep
	    if (sleep_debt >= c.SleepInterval)  {
		sleep_at = process.hrtime()
		return setTimeout(body_func, sleep_debt / millis_per_nano, r - 1)
	    }
	}
	var endTime = process.hrtime()
	var elapsed = (endTime[0] - begin[0]) + (endTime[1] - begin[1]) * 1e-9
	var done_func = function (err) {
	    // TODO check err
	    var flushDiff = process.hrtime(endTime);
	    var flushElapsed = flushDiff[0] + flushDiff[1] * 1e9
	    var path = '/result?timing=' + elapsed + '&flush=' + flushElapsed +
		'&a=' + p + '&s=' + sleep_nanos;
	    return http.get({
		host: host,
		port: port,
		path: path,
		agent: keepAliveAgent,
	    }, next_control)
	}
	if (c.Trace && !c.NoFlush) {
	    return tracer.flush(done_func)
	}
	return done_func(null)
    }
    body_func(c.Repeat)
}

next_control();
