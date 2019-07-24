import random
from http.server import BaseHTTPRequestHandler

class Histogram:
    def __init__(self, dict):
        random.seed() # uses os generated random as seed
        self.bins = []
        self.counts = []

        for bin in dict:
            self._add_bin(bin, dict[bin])

    def _bins_overlap(self, bin1, bin2):
        bin1_min, bin1_max = bin1
        bin2_min, bin2_max = bin2

        bin1_in_2 = (bin1_min >= bin2_min and bin1_min < bin2_max)
        bin2_in_1 = (bin2_min >= bin1_min and bin2_min < bin1_max)

        return bin1_in_2 or bin2_in_1

    """ min is inclusive, max is exclusive."""
    def _add_bin(self, new_bin, count):
        for bin in self.bins:
            if self._bins_overlap(bin, new_bin):
                raise Exception(f'tried to add bin {new_bin} which overlaps with existing bin {bin}')

        self.bins.append(new_bin)
        self.counts.append(count)

    def sample(self):
        total_rands = sum(self.counts)
        rand_num = random.randint(0, total_rands - 1) # generates in range [0, total_rands)
        bin_number = 0

        while True:
            rand_num -= self.counts[bin_number]

            if rand_num < 0:
                break

            bin_number += 1

        # return an average of the bin max and min
        return random.randint(self.bins[bin_number][0], self.bins[bin_number][1])


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
