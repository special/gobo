package fastsizer

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"io/ioutil"
)

type jpegHandler struct {
	r  *bufio.Reader
	re ImageInfo
}

func (h *jpegHandler) Info(r io.Reader) (ImageInfo, error) {
	h.r = bufio.NewReader(r)
	h.re.Type = JPEG

	for {
		// Discard until next marker
		_, err := h.r.ReadBytes(0xff)
		if err != nil {
			return h.re, err
		}
		marker, err := h.r.ReadByte()
		if err != nil {
			return h.re, err
		}
		if marker == 0 {
			continue
		}

		// Skip 0xff padding
		for marker == 0xff {
			marker, err = h.r.ReadByte()
			if err != nil {
				return h.re, err
			}
		}

		if marker == eoiMarker {
			return h.re, errors.New("end of image")
		} else if marker >= rst0Marker && marker <= rst7Marker {
			continue
		}

		// Read segment size
		buf := make([]byte, 2)
		if _, err := io.ReadFull(h.r, buf); err != nil {
			return h.re, err
		}
		length := int(buf[0])<<8 + int(buf[1]) - 2
		if length < 0 {
			return h.re, errors.New("short segment length")
		}

		switch marker {
		case sof0Marker, sof1Marker, sof2Marker:
			buf = make([]byte, 5)
			if _, err := io.ReadFull(h.r, buf); err != nil {
				return h.re, err
			} else if buf[0] != 8 {
				return h.re, errors.New("only 8-bit precision is supported")
			}

			h.re.Size = ImageSize{
				Width:  uint32(int(buf[3])<<8 + int(buf[4])),
				Height: uint32(int(buf[1])<<8 + int(buf[2])),
			}
			return h.re, nil

		case app1Marker:
			if err := h.readExif(length); err != nil {
				return h.re, nil
			}

		case sosMarker:
			return h.re, errors.New("reached sos marker")

		default:
			if _, err := h.r.Discard(length); err != nil {
				return h.re, err
			}
		}
	}
}

var errNotExif = errors.New("not exif")

// Adapted from https://github.com/disintegration/imageorient/
func (h *jpegHandler) readExif(n int) error {
	// EXIF marker
	const (
		markerSOI      = 0xffd8
		markerAPP1     = 0xffe1
		exifHeader     = 0x45786966
		byteOrderBE    = 0x4d4d
		byteOrderLE    = 0x4949
		orientationTag = 0x0112
	)

	// XXX This is a lazy way to avoid rewriting the logic
	buf := make([]byte, n)
	if _, err := io.ReadFull(h.r, buf); err != nil {
		return err
	}
	r := bytes.NewBuffer(buf)

	// Check if EXIF header is present.
	var header uint32
	if err := binary.Read(r, binary.BigEndian, &header); err != nil {
		return err
	}
	if header != exifHeader {
		return errNotExif
	}
	if _, err := io.CopyN(ioutil.Discard, r, 2); err != nil {
		return err
	}

	// Read byte order information.
	var (
		byteOrderTag uint16
		byteOrder    binary.ByteOrder
	)
	if err := binary.Read(r, binary.BigEndian, &byteOrderTag); err != nil {
		return err
	}
	switch byteOrderTag {
	case byteOrderBE:
		byteOrder = binary.BigEndian
	case byteOrderLE:
		byteOrder = binary.LittleEndian
	default:
		return errors.New("invalid byte order")
	}
	if _, err := io.CopyN(ioutil.Discard, r, 2); err != nil {
		return err
	}

	// Skip the EXIF offset.
	var offset uint32
	if err := binary.Read(r, byteOrder, &offset); err != nil {
		return err
	}
	if offset < 8 {
		return errors.New("invalid EXIF offset")
	}
	if _, err := io.CopyN(ioutil.Discard, r, int64(offset-8)); err != nil {
		return err
	}

	// Read the number of tags.
	var numTags uint16
	if err := binary.Read(r, byteOrder, &numTags); err != nil {
		return err
	}

	// Find the orientation tag.
	for i := 0; i < int(numTags); i++ {
		var tag uint16
		if err := binary.Read(r, byteOrder, &tag); err != nil {
			return err
		}
		if tag != orientationTag {
			if _, err := io.CopyN(ioutil.Discard, r, 10); err != nil {
				return err
			}
			continue
		}
		if _, err := io.CopyN(ioutil.Discard, r, 6); err != nil {
			return err
		}
		var val uint16
		if err := binary.Read(r, byteOrder, &val); err != nil {
			return err
		}
		if val < 1 || val > 8 {
			return errors.New("invalid orientation tag")
		}

		switch val {
		case 2:
			h.re.Mirror = MirrorHorizontal
		case 3:
			h.re.Rotation = 180
		case 4:
			h.re.Mirror = MirrorVertical
		case 5:
			h.re.Mirror = MirrorHorizontal
			fallthrough
		case 8:
			h.re.Rotation = 270
		case 7:
			h.re.Mirror = MirrorHorizontal
			fallthrough
		case 6:
			h.re.Rotation = 90
		}

		return nil
	}

	return nil
}

const (
	sof0Marker = 0xc0 // Start Of Frame (Baseline).
	sof1Marker = 0xc1 // Start Of Frame (Extended Sequential).
	sof2Marker = 0xc2 // Start Of Frame (Progressive).
	dhtMarker  = 0xc4 // Define Huffman Table.
	rst0Marker = 0xd0 // ReSTart (0).
	rst7Marker = 0xd7 // ReSTart (7).
	soiMarker  = 0xd8 // Start Of Image.
	eoiMarker  = 0xd9 // End Of Image.
	sosMarker  = 0xda // Start Of Scan.
	dqtMarker  = 0xdb // Define Quantization Table.
	driMarker  = 0xdd // Define Restart Interval.
	comMarker  = 0xfe // COMment.
	// "APPlication specific" markers aren't part of the JPEG spec per se,
	// but in practice, their use is described at
	// http://www.sno.phy.queensu.ca/~phil/exiftool/TagNames/JPEG.html
	app0Marker  = 0xe0
	app1Marker  = 0xe1
	app14Marker = 0xee
	app15Marker = 0xef
)
