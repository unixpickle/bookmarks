package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"

	"github.com/unixpickle/essentials"
)

func main() {
	f, err := os.Open("template.png")
	essentials.Must(err)
	defer f.Close()
	img, err := png.Decode(f)

	colors := []color.RGBA{
		{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		{R: 0x9a, G: 0x9a, B: 0x9a, A: 0xff},
		{R: 0x00, G: 0x00, B: 0x00, A: 0xff},
	}
	outputs := make([]*image.RGBA, len(colors)+1)
	for i := range outputs {
		outputs[i] = image.NewRGBA(img.Bounds())
	}
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			r, g, b, a := img.At(x, y).RGBA()
			r >>= 8
			g >>= 8
			b >>= 8
			a >>= 8
			if a < 0xf0 {
				continue
			}
			outputs[len(outputs)-1].Set(x, y, color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
			for i, c := range colors {
				diff := math.Abs(float64(c.R)-float64(r)) +
					math.Abs(float64(c.G)-float64(g)) +
					math.Abs(float64(c.B)-float64(b))
				if diff < 50 {
					outputs[i].Set(x, y, color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
				}
			}
		}
	}

	// The gray image has some very small spots on the border of
	// white, which we want to remove.
	whiteImage, grayImage := outputs[0], outputs[1]
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if _, _, _, a := grayImage.At(x, y).RGBA(); a < 0x80 {
				continue
			}
			found := false
			// Check for any white around us.
			for cx := x - 1; cx <= x+1; cx++ {
				if cx < 0 || cx >= img.Bounds().Dx() {
					continue
				}
				for cy := y - 1; cy <= y+1; cy++ {
					if cy < 0 || cy >= img.Bounds().Dy() {
						continue
					}
					if _, _, _, a := whiteImage.At(cx, cy).RGBA(); a > 0x80 {
						found = true
					}
				}
			}
			if found {
				grayImage.Set(x, y, color.RGBA{})
			}
		}
	}

	names := []string{"white", "grey", "black", "union"}
	for i, name := range names {
		part := outputs[i]
		w, err := os.Create(name + ".png")
		essentials.Must(err)
		png.Encode(w, part)
		w.Close()
	}
}
