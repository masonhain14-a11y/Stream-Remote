//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"syscall"
	"unsafe"
)

var getIPv4Table = syscall.NewLazyDLL("iphlpapi.dll").NewProc("GetIpAddrTable")

// Some adapters fail Windows' general interface query. The IPv4 table also
// provides the subnet masks needed to check that a phone is on the local network.
func fallbackNetworks() []address {
	const insufficientBuffer = 122
	const headerSize, rowSize = 4, 24
	var size uint32
	result, _, _ := getIPv4Table.Call(0, uintptr(unsafe.Pointer(&size)), 0)
	for range 3 {
		if result != insufficientBuffer || size < headerSize || size > 1<<20 {
			break
		}
		table := make([]byte, size)
		result, _, _ = getIPv4Table.Call(uintptr(unsafe.Pointer(&table[0])), uintptr(unsafe.Pointer(&size)), 0)
		if result != 0 {
			continue
		}
		count := int(binary.LittleEndian.Uint32(table))
		count = min(count, (len(table)-headerSize)/rowSize)
		list := make([]address, 0, count)
		for i := range count {
			offset := headerSize + i*rowSize
			row := table[offset : offset+rowSize]
			ip := netip.AddrFrom4([4]byte(row[:4]))
			if !usableIPv4(ip) {
				continue
			}
			bits, width := net.IPMask(row[8:12]).Size()
			if width != 32 || bits == 0 {
				continue
			}
			name := fmt.Sprintf("Network adapter %d", binary.LittleEndian.Uint32(row[4:8]))
			list = append(list, address{netip.PrefixFrom(ip, bits), name})
		}
		return list
	}
	return nil
}
