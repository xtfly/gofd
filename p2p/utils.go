package p2p

import (
	"encoding/binary"
	"fmt"
	"io"
)

func checkEqual(ref, current []byte) bool {
	for i := 0; i < len(current); i++ {
		if ref[i] != current[i] {
			return false
		}
	}
	return true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func uint32ToBytes(buf []byte, n uint32) {
	binary.BigEndian.PutUint32(buf, n)
}

func bytesToUint32(buf []byte) uint32 {
	return binary.BigEndian.Uint32(buf)
}

func writeNBOUint32(w io.Writer, n uint32) (err error) {
	buf := make([]byte, 4)
	uint32ToBytes(buf, n)
	_, err = w.Write(buf[0:])
	return
}

func readNBOUint32(r io.Reader) (n uint32, err error) {
	var buf [4]byte
	_, err = io.ReadFull(r, buf[0:])
	if err != nil {
		return
	}
	n = bytesToUint32(buf[0:])
	return
}

func humanSize(value float64) string {
	switch {
	case value > 1<<30:
		return fmt.Sprintf("%.2f GB", value/(1<<30))
	case value > 1<<20:
		return fmt.Sprintf("%.2f MB", value/(1<<20))
	case value > 1<<10:
		return fmt.Sprintf("%.2f kB", value/(1<<10))
	}
	return fmt.Sprintf("%.2f B", value)
}
