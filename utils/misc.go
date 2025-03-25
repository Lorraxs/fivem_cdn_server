package utils

import "lorraxs/fivem_cdn_server/structs"

func RgbaToPixel(r uint32, g uint32, b uint32, a uint32) structs.Pixel {
	return structs.Pixel{R: int(r / 257), G: int(g / 257), B: int(b / 257), A: int(a / 257)}
}
