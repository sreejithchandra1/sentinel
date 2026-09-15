//go:build !linux

package main

import "syscall"

func fsBlockSize(st syscall.Statfs_t) uint64 {
	if st.Bsize > 0 {
		return uint64(st.Bsize)
	}
	return 0
}

func fsBavail(st syscall.Statfs_t) uint64 {
	return uint64(st.Bavail)
}
