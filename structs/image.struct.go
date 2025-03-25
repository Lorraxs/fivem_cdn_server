package structs

import "image/color"

type Pixel struct {
	R int
	G int
	B int
	A int
}

func (p Pixel) ToRgba() color.RGBA {
	return color.RGBA{uint8(p.R), uint8(p.G), uint8(p.B), uint8(p.A)}
}
