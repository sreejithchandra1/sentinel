package main

func diskBytes(blocks, bavail, bsize uint64) (total, used, free uint64) {
	if blocks == 0 || bsize == 0 {
		return 0, 0, 0
	}
	if bsize > ^uint64(0)/blocks {
		return 0, 0, 0
	}
	total = blocks * bsize
	if bavail > blocks {
		bavail = blocks
	}
	if bsize > ^uint64(0)/bavail && bavail > 0 {
		return 0, 0, 0
	}
	free = bavail * bsize
	if free > total {
		free = total
	}
	used = total - free
	return total, used, free
}
