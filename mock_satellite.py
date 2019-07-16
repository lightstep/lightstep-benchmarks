import generated.collector_pb2 as collector
import google.protobuf
from http.server import BaseHTTPRequestHandler, HTTPServer, ThreadingHTTPServer
import binascii
import sys
import threading



spans_received = 0
global_lock = threading.Lock()


"""
A class that extends BaseHTTPRequestHandler to support chunked encoding. The
class will read POST request headers and determine if the request is in
fixed-length or chunked format. The request body will be parsed and saved
in @binary_body bytearray.

Derrived classes should use POST and GET instead of do_POST and do_GET.
"""
class ChunkedRequestHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        # if there is a content-length header, we know how much data to read
        if "Content-Length" in self.headers:
            content_length = int(self.headers["Content-Length"])
            self.binary_body = self.rfile.read(content_length)

        # if there is a chunked encoding,
        elif 'Transfer-Encoding' in self.headers and self.headers['Transfer-Encoding'] == 'chunked': # see http://en.wikipedia.org/wiki/Chunked_transfer_encoding
            self.binary_body = bytearray()

            while True:
                # chunk begind with [length hex]\r\n
                read_len = self._read_chunk_length()

                # when there is a 0-length chunk we are done
                if read_len <= 0:
                    break

                # read appropriate number of bytes
                binary_chunk = self.rfile.read(read_len)
                self.binary_body += binary_chunk

                # chunk ends with /r/n
                self._read_delimiter()

        else:
            raise Exception("POST request didn't have Content-Length or Transfer-Encoding headers")

        self.POST()

    def do_GET(self):
        self.GET()

    def POST(self):
        pass

    def GET(self):
        pass

    def _read_delimiter(self, delimiter=b'\r\n'):
        bytes_read = self.rfile.read(len(delimiter))

        if bytes_read != delimiter:
            raise Exception()

    def _read_chunk_length(self, delimiter=b'\r\n', max_bytes=16):
        buf = bytearray()
        delim_len = len(delimiter)

        while len(buf) < max_bytes:
            c = self.rfile.read(1)

            buf += c

            if buf[-delim_len:] == delimiter:

                try:
                    l = int(bytes(buf[:-delim_len]), 16)
                    # print("read", len(buf), "bytes:", l)
                    return l
                except ValueError:
                    return -1

        return -1

class SatelliteRequestHandler(ChunkedRequestHandler):
    def _send_response(self, response_code, body_string=None):
        self.send_response(response_code)

        if body_string:
            self.send_header("Content-Length", len(body_string))

        self.end_headers()

        if body_string:
            self.wfile.write((body_string).encode('utf-8'))

    """
    We want to delay mock satellites a little bit so that they have
    more realistic latency. Real latency numbers for gRPC are shown in this
    snapshot:

    https://app-meta.lightstep.com/lightstep-public/explorer?snapshot_id=bzB51UI9Nj
    """
    def _delay(self):
        pass

    # can't make a GET request to satellite server
    def GET(self):
        if self.path == "/spans_received":
            # don't need to worry about locking here since we're not going to
            # modify
            global spans_received
            self._send_response(200, body_string=str(spans_received))
            return
        else:
            self._send_response(400)


    def POST(self):
        print("starting: ", threading.get_ident())
        if self.path == "/api/v2/reports":
            report_request = collector.ReportRequest()
            try:
                report_request.ParseFromString(self.binary_body)
            except google.protobuf.message.DecodeError as e:
                # when satellites are unable to parse the report_request, they
                # send a 500 with a brief description
                self._send_response(500, str(e))
                return

            global spans_received
            spans_in_report = len(report_request.spans)

            # aquire the global variable lock because we are using a
            # "multithreaded" server
            with global_lock:
                spans_received += spans_in_report

            print("read", spans_in_report, "spans, total", spans_received)

            self._send_response(200)
        else:
            self._send_response(400)

        print("ending: ", threading.get_ident())


if __name__ == "__main__":
    server_address = ('', 8012)

    # although this can't use "real" threading because of GIL, it can switch to
    # execute something else when we are waiting on a synchronous syscall
    httpd = ThreadingHTTPServer(server_address, SatelliteRequestHandler)
    httpd.serve_forever()
