package nq

import (
	"net"
)

type printf = func(f string, v ...interface{})

// out is a Printf wrapper
func out(printf printf, f string, v ...interface{}) {
	if printf != nil {
		printf("nanoq: "+f, v...)
	}
}

// closeConn verbosely close the connection
func closeConn(printf printf, conn net.Conn, direction string) {
	if conn == nil {
		return
	}
	m := "close connection"
	if addr := conn.RemoteAddr(); addr != nil {
		m += " " + direction + " " + addr.String()
	}
	if err := conn.Close(); err != nil {
		m = "failed to " + m + ": " + err.Error()
	}
	out(printf, m)
}
