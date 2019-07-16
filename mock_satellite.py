import generated.collector_pb2 as collector
import google.protobuf
from http.server import BaseHTTPRequestHandler, HTTPServer
import binascii
import sys

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


spans_received = 0

class SatelliteRequestHandler(ChunkedRequestHandler):
    # def __init__(self, *args, **kwargs):
    #     self._spans_received = 0
    #
    #     super(SatelliteRequestHandler, self).__init__(*args, **kwargs)

    def _send_response(self, response_code):
        self.send_response(response_code)
        self.end_headers()

    # can't make a GET request to satellite server
    def GET(self):
        global spans_received

        if self.path == "/spans_received":
            message = str(spans_received).encode('utf-8')

            self.send_response(200)
            self.send_header("Content-Length", len(message))
            self.end_headers()

             # needs to be binary encoded because this is a binary stream
            self.wfile.write(message)
            return
        else:
            self._send_response(400)


    def POST(self):
        global spans_received

        if self.path == "/api/v2/reports":
            report_request = collector.ReportRequest()
            try:
                report_request.ParseFromString(self.binary_body)
            except google.protobuf.message.DecodeError as e:
                # TODO: what should satellites do when the report_request can't
                # be parsed?
                print("error: ", e)
                self._send_response(400)
                return


            spans_in_report = len(report_request.spans)
            spans_received += spans_in_report # TODO: is there a more threadsafe way to do this ??
            print("read", spans_in_report, "spans, total", spans_received)

            self._send_response(200)
        else:
            self._send_response(400)

def run():
    server_address = ('', 8012)
    httpd = HTTPServer(server_address, SatelliteRequestHandler)
    httpd.serve_forever()

    print("something here??")

if __name__ == "__main__":
    run()
