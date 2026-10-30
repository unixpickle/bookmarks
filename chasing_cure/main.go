package main

import (
	"image"
	"image/color"
	_ "image/png"
	"log"
	"math"
	"os"

	"github.com/unixpickle/essentials"
	"github.com/unixpickle/model3d/model2d"
	"github.com/unixpickle/model3d/model3d"
	"github.com/unixpickle/model3d/render3d"
	"github.com/unixpickle/model3d/toolbox3d"
)

const (
	ImagePath = "chasing_cure.png"
	OutPath   = "chasing_cure.obj"

	WidthMM = 1.5 * 25.4

	WhiteHeight = 1.4
	GreenHeight = WhiteHeight + 0.5
	BrownHeight = GreenHeight

	AlphaThreshold    = 0.12
	ContourDelta      = 0.15
	ColorZTolerance   = 0.20
	BottomZTolerance  = 0.08
	SmoothIters       = 12
	ColorOutsetPx     = 0.35
	SideColorOutsetPx = 3.0
	MeshZScale        = 4.0
)

type pixelClass int

const (
	classTransparent pixelClass = iota
	classWhite
	classBrown
	classGreen
)

func main() {
	img := mustReadImage(ImagePath)
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	scale := WidthMM / float64(width)

	log.Printf("image: %dx%d px", width, height)
	log.Printf("scale: %.6f mm/px; size: %.3f x %.3f mm", scale, WidthMM, float64(height)*scale)

	outlineBmp := bitmapForClass(img, func(c pixelClass) bool {
		return c != classTransparent
	}).FlipY()
	whiteBmp := bitmapForClass(img, func(c pixelClass) bool {
		return c == classWhite
	}).FlipY()
	brownBmp := bitmapForClass(img, func(c pixelClass) bool {
		return c == classBrown
	}).FlipY()

	outline := bitmapSolid(outlineBmp, 0)
	white := bitmapSolid(whiteBmp, ColorOutsetPx)
	brown := bitmapSolid(brownBmp, ColorOutsetPx)
	whiteSide := bitmapSolid(whiteBmp, SideColorOutsetPx)
	brownSide := bitmapSolid(brownBmp, SideColorOutsetPx)

	heightAt := func(c model2d.Coord) float64 {
		if !outline.Contains(c) {
			return 0
		}
		if white.Contains(c) {
			return WhiteHeight
		}
		if brown.Contains(c) {
			return BrownHeight
		}
		return GreenHeight
	}

	classAt := func(c model2d.Coord) pixelClass {
		if white.Contains(c) {
			return classWhite
		}
		if brown.Contains(c) {
			return classBrown
		}
		return classGreen
	}
	sideClassAt := func(c model2d.Coord) pixelClass {
		if brownSide.Contains(c) {
			return classBrown
		}
		if whiteSide.Contains(c) {
			return classWhite
		}
		return classGreen
	}

	solid := model3d.CheckedFuncSolid(
		model3d.Origin,
		model3d.XYZ(float64(width), float64(height), GreenHeight),
		func(c model3d.Coord3D) bool {
			return c.Z >= 0 && c.Z < heightAt(c.XY())
		},
	)

	colorFn := toolbox3d.CoordColorFunc(func(c model3d.Coord3D) render3d.Color {
		if math.Abs(c.Z) <= BottomZTolerance {
			return classColor(classGreen)
		}
		if nearHeight(c.Z, heightAt(c.XY())) {
			return classColor(classAt(c.XY()))
		}
		return classColor(sideClassAt(c.XY()))
	})

	finalXf := &model3d.VecScale{Scale: model3d.XYZ(scale, scale, 1)}
	meshXf := &model3d.VecScale{Scale: model3d.XYZ(scale, scale, MeshZScale)}
	xfSolid := model3d.TransformSolid(meshXf, solid)
	xfColorFn := colorFn.Transform(meshXf).Cached()

	log.Println("dual contouring...")
	mesh := model3d.DualContour(xfSolid, ContourDelta, true, false)
	log.Println("eliminating triangles...")
	mesh = mesh.EliminateCoplanarFiltered(1e-8, xfColorFn.ChangeFilterFunc(mesh, ContourDelta))
	mesh = mesh.Transform(&model3d.VecScale{Scale: model3d.XYZ(1, 1, 1/MeshZScale)})
	finalColorFn := colorFn.Transform(finalXf).Cached()
	log.Println("saving with colors...")
	essentials.Must(mesh.SaveVertexColorOBJ(OutPath, finalColorFn.SRGB))
	log.Printf("saved %s with %d triangles", OutPath, mesh.NumTriangles())
}

func mustReadImage(path string) image.Image {
	f, err := os.Open(path)
	essentials.Must(err)
	defer f.Close()
	img, _, err := image.Decode(f)
	essentials.Must(err)
	return img
}

func bitmapForClass(img image.Image, keep func(pixelClass) bool) *model2d.Bitmap {
	bounds := img.Bounds()
	bmp := model2d.NewBitmap(bounds.Dx(), bounds.Dy())
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			bmp.Set(x-bounds.Min.X, y-bounds.Min.Y, keep(classify(img.At(x, y))))
		}
	}
	return bmp
}

func bitmapSolid(bmp *model2d.Bitmap, outset float64) model2d.Solid {
	collider := bitmapCollider(bmp)
	if outset == 0 {
		return model2d.NewColliderSolid(collider)
	}
	return model2d.NewColliderSolidInset(collider, -outset)
}

func bitmapCollider(bmp *model2d.Bitmap) model2d.MultiCollider {
	return model2d.MeshToCollider(bmp.Mesh().SmoothSq(SmoothIters))
}

func nearHeight(z, height float64) bool {
	return math.Abs(z-height) <= ColorZTolerance
}

func classColor(c pixelClass) render3d.Color {
	switch c {
	case classWhite:
		return render3d.NewColor(1)
	case classBrown:
		return render3d.NewColorRGB(0.43, 0.20, 0.08)
	default:
		return render3d.NewColorRGB(0.00, 0.45, 0.16)
	}
}

func classify(c color.Color) pixelClass {
	r16, g16, b16, a16 := c.RGBA()
	a := float64(a16) / 0xffff
	if a < AlphaThreshold {
		return classTransparent
	}

	r := unpremul(float64(r16)/0xffff, a)
	g := unpremul(float64(g16)/0xffff, a)
	b := unpremul(float64(b16)/0xffff, a)

	if g > r*1.25 && g > b*1.25 {
		return classGreen
	}
	if r > g*1.25 && g > b*1.10 && b < 0.25 {
		return classBrown
	}
	return classWhite
}

func unpremul(x, a float64) float64 {
	if a == 0 {
		return 0
	}
	return math.Min(x/a, 1)
}
