package main

func hash32(key uint32) uint32 {
	// MurmurHash3 finalizer mix
	key ^= key >> 16
	key *= 0x85ebca6b
	key ^= key >> 13
	key *= 0xc2b2ae35
	key ^= key >> 16
	return key
}