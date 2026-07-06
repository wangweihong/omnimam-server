package netutil

import (
	"net"
)

func CheckPortUsed(port string) (bool, error) {
	address := ":" + port

	tcpAddr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return false, err
	}
	tcpSocket, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return true, err
	}
	tcpSocket.Close()
	return false, nil
}

func IsValidPort(port int) bool {
	if port <= 0 || port >= 65535 {
		return false
	}

	return true
}
