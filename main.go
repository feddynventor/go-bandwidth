package main

import (
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/mohae/struct2csv"
)

const (
	CONN_HOST = "0.0.0.0"
	CONN_PORT = "3000"
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

var statistics []stats

type stats struct {
	age        float64
	sent_bytes uint64
	tcp_info   syscall.TCPInfo
}

// Handles incoming requests.
func handleRequest(conn net.Conn) {
	var csv_enc = struct2csv.New()
	var total_bytes uint64
	statistics = nil // Reset statistics

	conn.Write([]byte("HTTP/1.1 200 OK\n\r\n\r"))
	start := time.Now()

	// Get the file descriptor of the connection
	fileDescriptor, err := conn.(*net.TCPConn).File()
	if err != nil {
		fmt.Println("Error getting file descriptor:", err)
		return
	}
	fd := int(fileDescriptor.Fd())

	// Stream a large response to the client.
	chunk := make([]byte, 1024*1024) // 1 MB

	for i := 0; i < 50; i++ {
		n, err := conn.Write(chunk)
		if err != nil {
			fmt.Printf("Error writing response: %v\n", err)
			break
		}

		total_bytes += uint64(n)

		// Get TCP statistics at every sent chunk
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

		// COPY! the TCPInfo struct to the statistics slice
		// https://docs.huihoo.com/doxygen/linux/kernel/3.7/include_2linux_2tcp_8h_source.html#l00128
		statistics = append(statistics, stats{
			sent_bytes: total_bytes,
			age:        time.Since(start).Seconds(),
			tcp_info:   tcpInfo,
		})

	}

	conn.Write([]byte("\n\r\n\r\n\r"))
	fmt.Printf("Sent %d bytes for %s\n", total_bytes, conn.RemoteAddr().String())

	for _, v := range statistics {
		// TODO: fix coincise way to convert struct to csv
		tcpinfo, _ := csv_enc.GetRow(v.tcp_info)
		conn.Write([]byte(fmt.Sprintf("%.6f", v.age) + "," + strings.Join(tcpinfo, ",") + "," + fmt.Sprintf("%d", v.sent_bytes) + "\n\r"))
	}

	fileDescriptor.Close()
	conn.Close()
	return
}
