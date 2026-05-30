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
	ImagePath = "key_v2.png"
	OutPath   = "key_v2.obj"

	WidthMM = 1.5 * 25.4

	WhiteHeight = 1.4
	GrayHeight  = WhiteHeight + 0.5
	BlackHeight = WhiteHeight + 1.0
	RedHeight   = WhiteHeight + 2.0

	SmooshCenter = GrayHeight
	SmooshScale  = 0.01
	SmooshCutoff = WhiteHeight + 0.20

	HoleRadiusMM    = 2.25
	HoleTopOffsetMM = 7.00

	AlphaThreshold   = 0.12
	GrayThreshold    = 0.90
	ContourDelta     = 0.15
	ColorZTolerance  = 0.20
	BottomZTolerance = 0.08
	EdgeTopTolerance = 0.06
	EdgeColorWidthMM = 0.50
	SmoothIters      = 12
	ColorOutsetPx    = 0.35
)

type pixelClass int

const (
	classTransparent pixelClass = iota
	classWhite
	classGray
	classBlack
	classRed
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
	blackBmp := bitmapForClass(img, func(c pixelClass) bool {
		return c == classBlack
	}).FlipY()
	redBmp := bitmapForClass(img, func(c pixelClass) bool {
		return c == classRed
	}).FlipY()
	grayBmp := bitmapForClass(img, func(c pixelClass) bool {
		return c == classGray
	}).FlipY()

	outlineCollider := bitmapCollider(outlineBmp)
	outline := model2d.NewColliderSolid(outlineCollider)
	black := bitmapSolid(blackBmp, ColorOutsetPx)
	red := bitmapSolid(redBmp, ColorOutsetPx)
	gray := bitmapSolid(grayBmp, ColorOutsetPx)
	hole := &model2d.Circle{
		Center: model2d.XY(float64(width)/2, float64(height)-HoleTopOffsetMM/scale),
		Radius: HoleRadiusMM / scale,
	}
	edge := model2d.JoinedSolid{
		model2d.NewColliderSolidHollow(outlineCollider, EdgeColorWidthMM/scale),
		model2d.NewColliderSolidHollow(hole, EdgeColorWidthMM/scale),
	}

	log.Printf("hole: center=(%.3f, %.3f) mm, radius=%.3f mm",
		hole.Center.X*scale, hole.Center.Y*scale, HoleRadiusMM)

	heightAt := func(c model2d.Coord) float64 {
		if !outline.Contains(c) || hole.Contains(c) {
			return 0
		}
		if red.Contains(c) {
			return RedHeight
		}
		if black.Contains(c) {
			return BlackHeight
		}
		if gray.Contains(c) {
			return GrayHeight
		}
		return WhiteHeight
	}

	solid := model3d.CheckedFuncSolid(
		model3d.Origin,
		model3d.XYZ(float64(width), float64(height), RedHeight),
		func(c model3d.Coord3D) bool {
			return c.Z >= 0 && c.Z < heightAt(c.XY())
		},
	)

	colorFn := toolbox3d.CoordColorFunc(func(c model3d.Coord3D) render3d.Color {
		if edge.Contains(c.XY()) {
			if topEdgeHeight(c.Z) {
				return classColor(classBlack)
			}
			return classColor(classWhite)
		}
		if math.Abs(c.Z) <= BottomZTolerance {
			return classColor(classWhite)
		}
		if heightClass, ok := classForHeight(c.Z); ok {
			return classColor(heightClass)
		}
		xyClass := colorAt(c.XY(), red, black, outline, hole)
		switch xyClass {
		case classRed, classBlack:
			return classColor(xyClass)
		default:
			return classColor(classWhite)
		}
	})

	xf := &model3d.VecScale{Scale: model3d.XYZ(scale, scale, 1)}
	xfSolid := model3d.TransformSolid(xf, solid)
	xfColorFn := colorFn.Transform(xf).Cached()

	log.Println("dual contouring...")
	mesh := model3d.DualContour(xfSolid, ContourDelta, true, false)
	log.Println("eliminating triangles...")
	mesh = mesh.EliminateCoplanarFiltered(1e-8, xfColorFn.ChangeFilterFunc(mesh, ContourDelta))
	log.Println("smooshing raised heights...")
	mesh = mesh.MapCoords(smooshCoord)
	xfColorFn = xfColorFn.Map(unsmooshCoord).Cached()
	log.Println("saving with colors...")
	essentials.Must(mesh.SaveVertexColorOBJ(OutPath, xfColorFn.SRGB))
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

func colorAt(c model2d.Coord, red, black, outline model2d.Solid, hole *model2d.Circle) pixelClass {
	if !outline.Contains(c) || hole.Contains(c) {
		return classTransparent
	}
	if red.Contains(c) {
		return classRed
	}
	if black.Contains(c) {
		return classBlack
	}
	return classWhite
}

func classForHeight(z float64) (pixelClass, bool) {
	if nearHeight(z, RedHeight) {
		return classRed, true
	}
	if nearHeight(z, BlackHeight) {
		return classBlack, true
	}
	if nearHeight(z, GrayHeight) || nearHeight(z, WhiteHeight) {
		return classWhite, true
	}
	return classTransparent, false
}

func nearHeight(z, height float64) bool {
	return math.Abs(z-height) <= ColorZTolerance
}

func topEdgeHeight(z float64) bool {
	return z >= WhiteHeight-EdgeTopTolerance
}

func classColor(c pixelClass) render3d.Color {
	switch c {
	case classRed:
		return render3d.NewColorRGB(0.82, 0.02, 0.08)
	case classBlack:
		return render3d.NewColor(0)
	default:
		return render3d.NewColor(1)
	}
}

func smooshCoord(c model3d.Coord3D) model3d.Coord3D {
	if c.Z > SmooshCutoff {
		c.Z = SmooshCenter + (c.Z-SmooshCenter)*SmooshScale
	}
	return c
}

func unsmooshCoord(c model3d.Coord3D) model3d.Coord3D {
	if c.Z > SmooshCutoff {
		c.Z = SmooshCenter + (c.Z-SmooshCenter)/SmooshScale
	}
	return c
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

	if r > 0.22 && r-math.Max(g, b) > 0.10 {
		return classRed
	}
	luminance := 0.2126*r + 0.7152*g + 0.0722*b
	if luminance < 0.38 {
		return classBlack
	}
	if luminance < GrayThreshold && maxComponent(r, g, b)-minComponent(r, g, b) < 0.10 {
		return classGray
	}
	return classWhite
}

func unpremul(x, a float64) float64 {
	if a == 0 {
		return 0
	}
	return math.Min(x/a, 1)
}

func minComponent(x, y, z float64) float64 {
	return math.Min(x, math.Min(y, z))
}

func maxComponent(x, y, z float64) float64 {
	return math.Max(x, math.Max(y, z))
}
