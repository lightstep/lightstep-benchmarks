/*
taken from: http://think-async.com/Asio/boost_asio_1_13_0/doc/html/boost_asio/tutorial/tutdaytime3/src.html
*/

#include <collector.pb.h>

#include <ctime>
#include <iostream>
#include <string>

#include <boost/bind.hpp>
#include <boost/shared_ptr.hpp>
#include <boost/enable_shared_from_this.hpp>
#include <boost/asio.hpp>
#include <boost/regex.hpp>
#include <boost/beast.hpp>

#include <sstream>

#define SERVER_PORT 1998
#define SATELLITE_IP "127.0.0.1"
#define SATELLITE_PORT "8360"
#define ACCESS_TOKEN "testtesttest"

using boost::asio::ip::tcp;
namespace beast = boost::beast;
namespace http = beast::http;
namespace net = boost::asio;

// Report a failure
void
fail(beast::error_code ec, char const* what)
{
    std::cerr << what << ": " << ec.message() << "\n";
}

// Performs an HTTP POST and prints the response
class session : public std::enable_shared_from_this<session>
{
    tcp::resolver resolver_;
    beast::tcp_stream stream_;
    beast::flat_buffer buffer_; // (Must persist between reads)
    http::request<http::string_body> req_;
    http::response<http::string_body> res_;

public:
    // Objects are constructed with a strand to
    // ensure that handlers do not execute concurrently.
    explicit
    session(net::io_context& ioc)
        : resolver_(net::make_strand(ioc))
        , stream_(net::make_strand(ioc))
    {
    }

    // Start the asynchronous operation
    void
    run(
        char const* host,
        char const* port,
        char const* target,
        int version,
        std::string body)
    {
        // Set up an HTTP POST request message
        req_.version(version);
        req_.method(http::verb::post);
        req_.target(target);
        req_.set(http::field::host, host);

        // standard headers for sending to satellites
        req_.insert("Content-Type", "application/octet-stream");
        req_.insert("Accept", "text/plain"); // get a human-readable response from the server
        req_.insert("Lightstep-Access-Token", ACCESS_TOKEN);

        req_.body() = body;
        req_.prepare_payload(); // sets body size in HTTP headers

        // Look up the domain name
        resolver_.async_resolve(
            host,
            port,
            beast::bind_front_handler(
                &session::on_resolve,
                shared_from_this()));
    }

    void
    on_resolve(
        beast::error_code ec,
        tcp::resolver::results_type results)
    {
        if(ec)
            return fail(ec, "resolve");

        // Set a timeout on the operation
        stream_.expires_after(std::chrono::seconds(30));

        // Make the connection on the IP address we get from a lookup
        stream_.async_connect(
            results,
            beast::bind_front_handler(
                &session::on_connect,
                shared_from_this()));
    }

    void
    on_connect(beast::error_code ec, tcp::resolver::results_type::endpoint_type)
    {
        if(ec)
            return fail(ec, "connect");

        // Set a timeout on the operation
        stream_.expires_after(std::chrono::seconds(30));

        // Send the HTTP request to the remote host
        http::async_write(stream_, req_,
            beast::bind_front_handler(
                &session::on_write,
                shared_from_this()));
    }

    void
    on_write(
        beast::error_code ec,
        std::size_t bytes_transferred)
    {
        boost::ignore_unused(bytes_transferred);

        if(ec)
            return fail(ec, "write");

        // Receive the HTTP response
        http::async_read(stream_, buffer_, res_,
            beast::bind_front_handler(
                &session::on_read,
                shared_from_this()));
    }

    void
    on_read(
        beast::error_code ec,
        std::size_t bytes_transferred)
    {
        boost::ignore_unused(bytes_transferred);

        if(ec)
            return fail(ec, "read");

        if (res_.result() != http::status::ok) {
          std::cerr << "*** received abnormal response ***" << std::endl;
          std::cerr << res_ << std::endl;
        } else {
          std::cout << "received normal response from satellite." << std::endl;
        }

        // Gracefully close the socket
        stream_.socket().shutdown(tcp::socket::shutdown_both, ec);

        // not_connected happens sometimes so don't bother reporting it.
        if(ec && ec != beast::errc::not_connected)
            return fail(ec, "shutdown");

        // If we get here then the connection is closed gracefully
    }
};

/*
we inherit from boost::enable_shared_from_this and support a shared_ptr
so that we can continue reusing a connection so long as
*/
class tcp_connection
  : public boost::enable_shared_from_this<tcp_connection>
{
private:
  tcp::socket socket_;
  boost::asio::streambuf request_data_;
  std::string message_;
  boost::asio::io_context &io_context_;

  beast::flat_buffer overflow_buf_; // resizable buffer of chars
  http::request<http::string_body> request_;

public:
  // make a nice name for a pointer to this class
  typedef boost::shared_ptr<tcp_connection> pointer;

  // create tcp_connection with this static factory-style method instead of a
  // constructor
  static pointer create(boost::asio::io_context& io_context)
  {
    return pointer(new tcp_connection(io_context));
  }

  tcp::socket& socket()
  {
    return socket_;
  }

  void start()
  {
    std::cout << "tcp_connection::start()" << std::endl;

    // read handler will write data into the read buffer
    auto read_handler = boost::bind(
      &tcp_connection::handle_read,
      shared_from_this(),
      boost::asio::placeholders::error,
      boost::asio::placeholders::bytes_transferred);


    request_ = {}; // clear the request before writing to it
    http::async_read(socket_, overflow_buf_, request_, read_handler);
  }

private:
  // private constructor because we are using enable_shared_from_this
  tcp_connection(boost::asio::io_context &io_context)
    : socket_(io_context),
      io_context_(io_context)
  {
  }

  void handle_write(const boost::system::error_code& /*error*/, size_t /*bytes_transferred*/)
  {
  }

  void handle_read(const boost::system::error_code& error, size_t bytes_transferred)
  {
    if(error == http::error::end_of_stream) {
      std::cout << "end of stream." << std::endl;
      return;
    }

    if(error) {
      std::cerr << "error reading : " << error << std::endl;
      return;
    }

    if (request_.base().method_string() != "POST") {
      std::cerr << "sidecar only accepts POST requests from tracers, not '"
        << request_.base().method_string() << "' requests." << std::endl;
      return;
    }

    /*
    parse the message to make sure that
      1) it has some spans in it
      2) it is valid protobuf !
    before we forward to satellite
    */

    // convert the request body from string --> stringstream
    GOOGLE_PROTOBUF_VERIFY_VERSION;

    std::stringstream body_stream(request_.body());
    lightstep::collector::ReportRequest report_request;

    if (!report_request.ParseFromIstream(&body_stream)) {
      std::cerr << "there was an error parsing the report request" << std::endl;
      return;
    }


    if (report_request.spans().size() != 0) {
      /*
      pass string by value because this is a simple first implementatiokn
      to actually make this performant we will want to stop some of this copying

      all of the retry logic and stuff should be isolated to this class

      11 == HTTP/1.1
      */
      std::cout << "sending " << report_request.spans().size() << " spans" << std::endl;
      std::make_shared<session>(io_context_)->run(SATELLITE_IP, SATELLITE_PORT, "/api/v2/reports", 11, request_.body());
    }

    write_response("200 OK");
  }

  void write_response(std::string status_line)
  {
    message_ = "HTTP/1.1 " + status_line + "\r\n\r\n";

    auto write_handler = boost::bind(
      &tcp_connection::handle_write,
      // passes 'this' in a way that the shared pointer class notices
      shared_from_this(),
      boost::asio::placeholders::error,
      boost::asio::placeholders::bytes_transferred);

    boost::asio::async_write(socket_, boost::asio::buffer(message_), write_handler);
  }
};


class tcp_server
{
private:
  boost::asio::io_context& io_context_;
  tcp::acceptor acceptor_;

public:
  tcp_server(boost::asio::io_context& io_context)
    : io_context_(io_context),
      // connect on every adddress on this machine
      acceptor_(io_context, tcp::endpoint(tcp::v4(), SERVER_PORT))
  {
    std::cout << "tcp_server::tcp_server()" << std::endl;

    start_accept();
  }

private:
  void start_accept()
  {
    std::cout << "tcp_server::start_accept()" << std::endl;

    // create a new socket that we can connect on
    tcp_connection::pointer new_connection =  tcp_connection::create(io_context_);

    // bind an accept handler to the new socket
    acceptor_.async_accept(
      new_connection->socket(),
      boost::bind(&tcp_server::handle_accept, this, new_connection, boost::asio::placeholders::error));
  }

  void handle_accept(tcp_connection::pointer new_connection, const boost::system::error_code& error)
  {
    std::cout << "tcp_server::handle_accept()" << std::endl;

    if (!error) {
      new_connection->start();
    }

    start_accept();
  }
};

int main() {
  try
  {
    boost::asio::io_context io_context;
    tcp_server server(io_context);
    io_context.run();
  }
  catch (std::exception &e)
  {
    std::cerr << e.what() << std::endl;
  }

  return 0;
}
