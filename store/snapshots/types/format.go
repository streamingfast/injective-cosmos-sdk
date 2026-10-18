package types

// CurrentFormat is the currently used format for snapshots. Snapshots using the same format
// must be identical across all nodes for a given height, so this must be bumped when the binary
// snapshot output changes.
//
// Format 4: binaries built with Go 1.27 produce different zlib-compressed chunk bytes than
// format 3 (Go <= 1.26) for the same state, because compress/flate changed. The decompressed
// snapshot stream is identical.
const CurrentFormat uint32 = 4

// formatGo126 is the format produced by binaries built with Go <= 1.26. It differs from
// CurrentFormat only in the zlib encoding of the chunks.
const formatGo126 uint32 = 3

// IsRestorableFormat reports whether a snapshot in the given format can be restored. Format 3
// snapshots restore identically, so nodes can still state-sync from snapshot providers that
// run a Go <= 1.26 build.
func IsRestorableFormat(format uint32) bool {
	return format == CurrentFormat || format == formatGo126
}
