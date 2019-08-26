package fastsizer

import (
	"io"
)

type bmpHandler struct{}

func (h *bmpHandler) Info(r io.Reader) (ImageInfo, error) {
	var info ImageInfo
	// Technically 18, but 2 bytes were read by detect
	// XXX that is sketchy.
	buf := make([]byte, 16+8)
	if _, err := io.ReadFull(r, buf); err != nil {
		return info, err
	}

	info.Size = ImageSize{
		Width:  uint32(readUint32(buf[0:4])),
		Height: uint32(readUint32(buf[4:8])),
	}
	return info, nil
}
