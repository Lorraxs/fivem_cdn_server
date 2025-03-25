package utils

import (
	"fmt"
	"lorraxs/fivem_cdn_server/structs"
	"path"
	"strings"
)

func RgbaToPixel(r uint32, g uint32, b uint32, a uint32) structs.Pixel {
	return structs.Pixel{R: int(r / 257), G: int(g / 257), B: int(b / 257), A: int(a / 257)}
}

func JoinURL(base string, paths ...string) string {
	p := path.Join(paths...)
	return fmt.Sprintf("%s/%s", strings.TrimRight(base, "/"), strings.TrimLeft(p, "/"))
}
