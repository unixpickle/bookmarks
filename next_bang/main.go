package main

import (
	"log"
	"math"

	"github.com/unixpickle/essentials"
	"github.com/unixpickle/model3d/model2d"
	"github.com/unixpickle/model3d/model3d"
	"github.com/unixpickle/model3d/render3d"
	"github.com/unixpickle/model3d/toolbox3d"
)

const (
	Thickness = 1.4
	Inlay     = 0.4
	XYScale   = 0.1
	CornerRad = 10.0
)

func main() {
	outlineBmp := model2d.MustReadBitmap("next_bang.png", nil).FlipY()
	inlayOutline := outsetSolid(outlineBmp, 0)
	width := float64(outlineBmp.Width)
	height := float64(outlineBmp.Height)
	solid := model3d.CheckedFuncSolid(
		model3d.Origin,
		model3d.XYZ(width, height, Thickness),
		func(c model3d.Coord3D) bool {
			if !containsRoundedRect(c.XY(), width, height, CornerRad) {
				return false
			}
			if inlayOutline.Contains(c.XY()) {
				return c.Z < Thickness-Inlay
			} else {
				return c.Z < Thickness
			}
		},
	)
	outsetInlayOutline := outsetSolid(outlineBmp, 3)
	colorFn := toolbox3d.CoordColorFunc(func(c model3d.Coord3D) render3d.Color {
		if math.Abs(c.Z-(Thickness-Inlay)) < 0.1 && outsetInlayOutline.Contains(c.XY()) {
			return render3d.NewColor(1)
		}
		return render3d.NewColor(0)
	})

	xf := &model3d.VecScale{Scale: model3d.XYZ(XYScale, XYScale, 1)}
	xfColorFn := colorFn.Transform(xf)
	xfSolid := model3d.TransformSolid(xf, solid)

	log.Println("dual contouring...")
	mesh := model3d.DualContour(xfSolid, 0.1, true, false)
	log.Println("eliminating triangles...")
	mesh = mesh.EliminateCoplanarFiltered(1e-8, xfColorFn.ChangeFilterFunc(mesh, 0.1))
	log.Println("saving with colors...")
	essentials.Must(mesh.SaveVertexColorOBJ("next_bang.obj", xfColorFn.SRGB))
}

// Avoid wrong color at/near edges using an outset.
func outsetSolid(bmp *model2d.Bitmap, outset float64) model2d.Solid {
	return model2d.NewColliderSolidInset(
		model2d.MeshToCollider(bmp.Mesh().SmoothSq(10)),
		-outset,
	)
}

func containsRoundedRect(c model2d.Coord, width, height, radius float64) bool {
	if c.X < 0 || c.Y < 0 || c.X > width || c.Y > height {
		return false
	}
	r := math.Min(radius, math.Min(width, height)/2)
	if r <= 0 {
		return true
	}
	if c.X >= r && c.X <= width-r {
		return true
	}
	if c.Y >= r && c.Y <= height-r {
		return true
	}
	cornerX := r
	if c.X > width/2 {
		cornerX = width - r
	}
	cornerY := r
	if c.Y > height/2 {
		cornerY = height - r
	}
	return c.Dist(model2d.XY(cornerX, cornerY)) <= r
}
