package netutil

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os/exec"
	"strings"
)

// GetMacIPFromARPCache 从ARP缓存表获取mac地址的ip
func GetMacIPFromARPCache(mac string) (string, error) {
	// Execute the `arp -a` command to get the ARP table
	cmd := exec.Command("arp", "-a")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, mac) {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				return fields[1], nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", fmt.Errorf("MAC address not found in ARP table")
}

const (
	ARP_REQUEST = 1
	ARP_REPLY   = 2
)

type ARPHeader struct {
	HardwareType       uint16
	ProtocolType       uint16
	HardwareAddrLength uint8
	ProtocolAddrLength uint8
	Operation          uint16
	SenderHardwareAddr [6]byte
	SenderProtocolAddr [4]byte
	TargetHardwareAddr [6]byte
	TargetProtocolAddr [4]byte
}

func GetMacIPFromARPBroadcast(targetMAC string) (string, error) {
	targetMACBytes, err := net.ParseMAC(targetMAC)
	if err != nil {
		return "", fmt.Errorf("error parsing MAC address:%v", err)
	}

	iface, err := GetDefaultInterface()
	if err != nil {
		return "", fmt.Errorf("error getting default interface:%v", err)
	}

	conn, err := net.ListenPacket("ethernet", iface.Name)
	if err != nil {
		return "", fmt.Errorf("error creating raw socket:%v", err)
	}
	defer conn.Close()

	srcIP := net.IPv4(0, 0, 0, 0)            // ARP request usually starts with source IP 0.0.0.0
	targetIP := net.IPv4(255, 255, 255, 255) // Broadcast IP address

	arpReq := ARPHeader{
		HardwareType:       1,      // Ethernet
		ProtocolType:       0x0800, // IPv4
		HardwareAddrLength: 6,
		ProtocolAddrLength: 4,
		Operation:          ARP_REQUEST,
	}
	copy(arpReq.SenderHardwareAddr[:], iface.HardwareAddr)
	copy(arpReq.SenderProtocolAddr[:], srcIP.To4())
	copy(arpReq.TargetHardwareAddr[:], targetMACBytes)
	copy(arpReq.TargetProtocolAddr[:], targetIP.To4())

	// Create Ethernet frame
	ethFrame := make([]byte, 14+28)
	copy(ethFrame[0:6], targetMACBytes)      // Destination MAC
	copy(ethFrame[6:12], iface.HardwareAddr) // Source MAC
	ethFrame[12] = 0x08
	ethFrame[13] = 0x06 // ARP protocol type

	// Fill ARP request in Ethernet frame
	buf := make([]byte, binary.Size(arpReq))
	binary.BigEndian.PutUint16(buf[0:2], arpReq.HardwareType)
	binary.BigEndian.PutUint16(buf[2:4], arpReq.ProtocolType)
	buf[4] = arpReq.HardwareAddrLength
	buf[5] = arpReq.ProtocolAddrLength
	binary.BigEndian.PutUint16(buf[6:8], arpReq.Operation)
	copy(buf[8:14], arpReq.SenderHardwareAddr[:])
	copy(buf[14:18], arpReq.SenderProtocolAddr[:])
	copy(buf[18:24], arpReq.TargetHardwareAddr[:])
	copy(buf[24:28], arpReq.TargetProtocolAddr[:])

	copy(ethFrame[14:], buf)

	// Send ARP request
	_, err = conn.WriteTo(ethFrame, &net.UDPAddr{IP: net.IPv4bcast})
	if err != nil {
		return "", fmt.Errorf("error sending ARP request:%v", err)
	}

	// Listen for ARP reply
	buf = make([]byte, 1500)
	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			return "", fmt.Errorf("error reading from socket:%v", err)
		}

		// Parse Ethernet frame
		if n < 42 || buf[12] != 0x08 || buf[13] != 0x06 {
			continue // Not an ARP packet
		}

		arp := ARPHeader{}
		arp.HardwareType = binary.BigEndian.Uint16(buf[14:16])
		arp.ProtocolType = binary.BigEndian.Uint16(buf[16:18])
		arp.HardwareAddrLength = buf[18]
		arp.ProtocolAddrLength = buf[19]
		arp.Operation = binary.BigEndian.Uint16(buf[20:22])
		copy(arp.SenderHardwareAddr[:], buf[22:28])
		copy(arp.SenderProtocolAddr[:], buf[28:32])
		copy(arp.TargetHardwareAddr[:], buf[32:38])
		copy(arp.TargetProtocolAddr[:], buf[38:42])

		if arp.Operation == ARP_REPLY && net.HardwareAddr(arp.SenderHardwareAddr[:]).String() == targetMAC {
			return net.IP(arp.SenderProtocolAddr[:]).String(), nil
		}
	}
}
