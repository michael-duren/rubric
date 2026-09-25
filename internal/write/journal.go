package write

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
)

type entry struct {
	path    string
	existed bool
	old     []byte
	oldMode fs.FileMode
	written string
	removed bool
}

type journal struct {
	entries []entry
	dirs    []string
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
