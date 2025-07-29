package gfx

import (
	"image"
	"strings"

	"github.com/BourgeoisBear/rasterm"
)

type Mode int

const (
	Sixel Mode = iota
	Iterm
	Kitty
	Unsupported
)

var SupportedMode Mode

func init() {
	sixel, _ := rasterm.IsSixelCapable()
	iterm := rasterm.IsItermCapable()
	kitty := rasterm.IsKittyCapable()
	if sixel {
		SupportedMode = Sixel
	} else if iterm {
		SupportedMode = Iterm
	} else if kitty {
		SupportedMode = Kitty
	} else {
		SupportedMode = Unsupported
	}
}

func PrintRGBA(i *image.Paletted) string {
	var buf strings.Builder

	switch SupportedMode {
	case Sixel:
		err := rasterm.SixelWriteImage(&buf, i)
		if err != nil {
			panic(err)
		}
	case Iterm:
		err := rasterm.ItermWriteImage(&buf, i)
		if err != nil {
			panic(err)
		}
	case Kitty:
		rasterm.KittyWriteImage(&buf, i, rasterm.KittyImgOpts{
			SrcX:        0,
			SrcY:        0,
			SrcWidth:    uint32(i.Bounds().Dx()),
			SrcHeight:   uint32(i.Bounds().Dy()),
			CellOffsetX: 0,
			CellOffsetY: 0,
			DstCols:     20,
			DstRows:     10,
			ZIndex:      0,
			ImageId:     0,
			ImageNo:     0,
			PlacementId: 0,
		})
	default:
		return "UNSUPPORTED TERMINAL"
	}
	return buf.String()
}
