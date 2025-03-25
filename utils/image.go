package utils

import (
	"fmt"
	"image"
	"image/color"
)

func RemoveGreenBackground(img image.Image) (image.Image, error) {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	newImage := image.NewRGBA(bounds)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			pixel := RgbaToPixel(r, g, b, a)
			if pixel.R+pixel.B < pixel.G {
				pixel.A = 0
			}
			newImage.Set(x, y, pixel.ToRgba())
		}
	}

	trimmedImage, err := TrimImage(newImage, 20)

	return trimmedImage, err
}

func TrimImage(img image.Image, minSize int) (image.Image, error) {
	fmt.Println("Trimming image...")
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	newImage := image.NewRGBA(bounds)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			newImage.Set(x, y, img.At(x, y))
		}
	}

	removeSmallParticles(newImage, minSize)

	minX, minY, maxX, maxY := findBoundingBox(newImage)
	fmt.Printf("minX: %d, minY: %d, maxX: %d, maxY: %d\n", minX, minY, maxX, maxY)

	trimmedImage := image.NewRGBA(image.Rect(0, 0, maxX-minX, maxY-minY))
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			trimmedImage.Set(x-minX, y-minY, newImage.At(x, y))
		}
	}

	return trimmedImage, nil
}

func removeSmallParticles(img *image.RGBA, minSize int) {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0 {
				if isSmallParticle(img, x, y, minSize, 0, 1) {
					removeParticle(img, x, y, minSize)
				}
			}
		}
	}

	for y := 0; y < height; y++ {
		for x := width - 1; x >= 0; x-- {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0 {
				if isSmallParticle(img, x, y, minSize, -1, 0) {
					removeParticle(img, x, y, minSize)
				}
			}
		}
	}

	for x := 0; x < width; x++ {
		for y := height - 1; y >= 0; y-- {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0 {
				if isSmallParticle(img, x, y, minSize, 0, -1) {
					removeParticle(img, x, y, minSize)
				}
			}
		}
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0 {
				if isSmallParticle(img, x, y, minSize, 1, 0) {
					removeParticle(img, x, y, minSize)
				}
			}
		}
	}
}

func isSmallParticle(img *image.RGBA, x, y, minSize, dx, dy int) bool {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	for i := 0; i < minSize; i++ {
		nx, ny := x+i*dx, y+i*dy
		if nx < 0 || nx >= width || ny < 0 || ny >= height {
			return false
		}
		if _, _, _, a := img.At(nx, ny).RGBA(); a != 0 {
			return false
		}
	}

	return true
}

func removeParticle(img *image.RGBA, x, y, minSize int) {
	for dy := 0; dy < minSize; dy++ {
		for dx := 0; dx < minSize; dx++ {
			img.Set(x+dx, y+dy, color.RGBA{0, 0, 0, 0})
		}
	}
}

func findBoundingBox(img *image.RGBA) (minX, minY, maxX, maxY int) {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	minX, minY = width, height
	maxX, maxY = 0, 0

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a != 0 {
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x > maxX {
					maxX = x
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}

	return
}
