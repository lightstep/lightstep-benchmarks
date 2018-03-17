// Compute the cost of sending a small UDP packet.
package main

import (
	"math/rand"
	"net"

	"github.com/lightstep/lightstep-benchmarks/experimental/diffbench"
)

// The packet size
const sendSize = 200

// udpSend is the function being measured.
func udpSend(sendBuffer []byte, id int32, conn *net.UDPConn) {
	// Prevent the compiler from observing the unused variable.
	sendBuffer[0] = byte(id & 0xff)
	if n, err := conn.Write(sendBuffer); err != nil || n != len(sendBuffer) {
		panic(err.Error())
	}
}

// connectUDP returns a connection for testing with.
func connectUDP() *net.UDPConn {
	// Note: /255 is a broadcast address, this prevents the
	// connection from failure (assumes netmask is /24).
	address := "192.168.0.255:8765"

	raddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		panic(err.Error())
	}

	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		panic(err.Error())
	}
	return conn
}

func main() {
	sendBuffer := make([]byte, sendSize)

	for i := range sendBuffer {
		sendBuffer[i] = byte(rand.Intn(256))
	}

	conn := connectUDP()
	test := func(id int32) { udpSend(sendBuffer, id, conn) }

	diffbench.RunAndSave("output", test)
}
