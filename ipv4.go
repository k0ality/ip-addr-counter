package main

import "encoding/binary"

func convertIPv4(ip string) (uint32, bool) {
	var octets [4]byte
	octetIdx := 0
	currentOctet := 0

	for i := 0; i < len(ip); i++ {
		c := ip[i]
		if c >= '0' && c <= '9' {
			currentOctet = currentOctet * 10 + int(c - '0')
			if currentOctet > 255 {
				return 0, false
			}
		} else if c == '.' {
			if octetIdx >= 3 {
				return 0, false
			}
			octets[octetIdx] = byte(currentOctet)
			octetIdx++
			currentOctet = 0
		} else {
			return 0, false
		}
	}

	if octetIdx != 3 {
		return 0, false
	}
	octets[3] = byte(currentOctet)

	return binary.BigEndian.Uint32(octets[:]), true
}