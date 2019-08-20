package fastsizer

import (
	"errors"
	"fmt"
	"io"
)

type ImageInfo struct {
	Size     ImageSize
	Type     ImageType
	Rotation int
	Mirror   MirrorDirection
}

func Detect(r io.Reader) (ImageInfo, error) {
	var re ImageInfo
	handler, err := detectTypeHandler(r)
	if err != nil {
		return re, err
	}

	return handler.Info(r)
}

type typeHandler interface {
	Info(r io.Reader) (ImageInfo, error)
}

func detectTypeHandler(r io.Reader) (typeHandler, error) {
	buf := make([]byte, 2)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}

	if buf[0] == 'B' && buf[1] == 'M' {
		// BMP
	} else if buf[0] == 0x47 && buf[1] == 0x49 {
		// GIF
	} else if buf[0] == 0xff && buf[1] == 0xd8 {
		// JPEG
		return &jpegHandler{}, nil
	} else if buf[0] == 0x89 && buf[1] == 0x50 {
		// PNG
	} else if (buf[0] == 'I' && buf[1] == 'I') || (buf[0] == 'M' && buf[1] == 'M') {
		// TIFF
	} else if buf[0] == 'R' && buf[1] == 'I' {
		// WEBP
	} else {
		return nil, errors.New("unknown image type")
	}

	return nil, errors.New("not implemented")
}

// FastImage instance needs to be initialized before use
type FastImage struct {
	tb             []byte
	internalBuffer []byte
}

// NewFastSizer returns a FastImage client
func NewFastSizer() *FastImage {
	return &FastImage{tb: make([]byte, 2), internalBuffer: make([]byte, 0, 2)}
}

type decoder struct {
	reader  *xbuffer
	minimal bool
}

//Detect image type and size
func (this *FastImage) Detect(reader io.Reader) (ImageType, ImageSize, error) {
	this.internalBuffer = this.internalBuffer[0:0]
	d := &decoder{
		reader:  newXbuffer(reader, this.internalBuffer),
		minimal: true,
	}
	info, err := this.detectInternal(d, reader)
	return info.Type, info.Size, err
}

func (this *FastImage) DetectInfo(reader io.Reader) (ImageInfo, error) {
	this.internalBuffer = this.internalBuffer[0:0]
	d := &decoder{reader: newXbuffer(reader, this.internalBuffer)}
	return this.detectInternal(d, reader)
}

func (this *FastImage) detectInternal(d *decoder, reader io.Reader) (ImageInfo, error) {
	var info ImageInfo
	var e error

	if _, err := d.reader.ReadAt(this.tb, 0); err != nil {
		return info, err
	}

	ok := false

	switch this.tb[0] {
	case 'B':
		switch this.tb[1] {
		case 'M':
			info.Type = BMP
			info.Size, e = d.getBMPImageSize()
			ok = true
		}
	case 0x47:
		switch this.tb[1] {
		case 0x49:
			info.Type = GIF
			info.Size, e = d.getGIFImageSize()
			ok = true
		}
	case 0x89:
		switch this.tb[1] {
		case 0x50:
			info.Type = PNG
			info.Size, e = d.getPNGImageSize()
			ok = true
		}
	case 'I':
		switch this.tb[1] {
		case 'I':
			info.Type = TIFF
			info.Size, e = d.getTIFFImageSize()
			ok = true
		}
	case 'M':
		switch this.tb[1] {
		case 'M':
			info.Type = TIFF
			info.Size, e = d.getTIFFImageSize()
			ok = true
		}
	case 'R':
		switch this.tb[1] {
		case 'I':
			info.Type = WEBP
			info.Size, e = d.getWEBPImageSize()
			ok = true
		}
	}

	this.internalBuffer = d.reader.buf

	if !ok {
		return info, fmt.Errorf("Unknown image type (%v)", this.tb)
	}
	return info, e
}
