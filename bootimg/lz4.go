package bootimg

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/pierrec/lz4/v4"
)

const (
	lz4LegacyMagic = 0x184c2102
	lz4LegacyChunk = 8 << 20
)

func IsLZ4Legacy(b []byte) bool {
	return len(b) >= 4 && binary.LittleEndian.Uint32(b) == lz4LegacyMagic
}

// LZ4Legacy compresses data in the legacy lz4 framing that the kernel's
// boot wrapper and Android bootloaders unpack.
func LZ4Legacy(data []byte) ([]byte, error) {
	var out bytes.Buffer
	out.Write(binary.LittleEndian.AppendUint32(nil, lz4LegacyMagic))
	buf := make([]byte, lz4.CompressBlockBound(lz4LegacyChunk))
	for off := 0; off < len(data); off += lz4LegacyChunk {
		chunk := data[off:min(off+lz4LegacyChunk, len(data))]
		n, err := lz4.CompressBlockHC(chunk, buf, lz4.Level9, nil, nil)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return nil, fmt.Errorf("lz4: chunk at %d does not compress", off)
		}
		out.Write(binary.LittleEndian.AppendUint32(nil, uint32(n)))
		out.Write(buf[:n])
	}
	return out.Bytes(), nil
}

func unLZ4Legacy(data []byte) ([]byte, error) {
	if !IsLZ4Legacy(data) {
		return nil, fmt.Errorf("not legacy lz4")
	}
	var out []byte
	buf := make([]byte, lz4LegacyChunk)
	for p := 4; p < len(data); {
		n := int(binary.LittleEndian.Uint32(data[p:]))
		p += 4
		if p+n > len(data) {
			return nil, fmt.Errorf("lz4: truncated chunk")
		}
		m, err := lz4.UncompressBlock(data[p:p+n], buf)
		if err != nil {
			return nil, err
		}
		out = append(out, buf[:m]...)
		p += n
	}
	return out, nil
}
