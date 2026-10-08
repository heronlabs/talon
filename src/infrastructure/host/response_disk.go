package host

// Disk is the filesystem of the root of the host from statfs, in bytes: its total and its free blocks, the reserve of root included.
type Disk struct {
	Total int64
	Free  int64
}
