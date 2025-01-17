package main

import (
	"fmt"
	"net"
	"os"
	"syscall"
	"time"
	"unsafe"
)

const (
	CONN_HOST = "localhost"
	CONN_PORT = "3333"
	CONN_TYPE = "tcp" // "tcp", "tcp4", "tcp6", "unix" or "unixpacket"
)

func main() {
	// Listen for incoming connections.
	l, err := net.Listen(CONN_TYPE, CONN_HOST+":"+CONN_PORT)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}
	// Close the listener when the application closes.
	defer l.Close()
	fmt.Println("Listening on " + CONN_HOST + ":" + CONN_PORT)
	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}
		// Handle connections in a new goroutine.
		go handleRequest(conn)
	}
}

// Handles incoming requests.
func handleRequest(conn net.Conn) {
	defer conn.Close()
	start := time.Now()
	var sentBytes int64

	// Get the file descriptor of the connection
	file, err := conn.(*net.TCPConn).File()
	if err != nil {
		fmt.Println("Error getting file descriptor:", err)
		return
	}
	fd := int(file.Fd())
	defer file.Close()

	// Stream a large response to the client.
	chunk := make([]byte, 1024*1024) // 1 MB
	for {
		n, err := conn.Write(chunk)
		if err != nil {
			fmt.Printf("Error writing response: %v\n", err)
			break
		}
		sentBytes += int64(n)

		// Measure time based bandwidth
		duration := time.Since(start).Seconds()
		bandwidth := float64(sentBytes) / duration / (1024 * 1024) // Mbps

		if (sentBytes % (1024 * 1024 * 10)) == 0 {
			// Get TCP statistics
			var tcpInfo syscall.TCPInfo
			size := uint32(unsafe.Sizeof(tcpInfo))
			_, _, errno := syscall.Syscall6(
				syscall.SYS_GETSOCKOPT,
				uintptr(fd),
				uintptr(syscall.SOL_TCP),
				uintptr(syscall.TCP_INFO),
				uintptr(unsafe.Pointer(&tcpInfo)),
				uintptr(unsafe.Pointer(&size)),
				0,
			)
			if errno != 0 {
				fmt.Println("Error getting TCP info:", errno)
				return
			}

			// Print TCP statistics
			// https://docs.huihoo.com/doxygen/linux/kernel/3.7/include_2linux_2tcp_8h_source.html#l00128
			fmt.Printf("Computed: %.2f\n %+v", bandwidth, tcpInfo)
		}

	}
}
