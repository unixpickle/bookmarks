package main

import (
	"image/color"
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
	XYScale   = 0.09
)

func main() {
	outlineBmp := model2d.MustReadBitmap("heart_the_lover.png", func(c color.Color) bool {
		_, _, b, _ := c.RGBA()
		return b < 0xffff/2
	}).FlipY()
	heartsOnly := outsetSolid(
		model2d.MustReadBitmap("heart_the_lover.png", func(c color.Color) bool {
			r, _, b, _ := c.RGBA()
			return b < 0xffff/3 && r > 0xffff/2
		}).FlipY(),
	)
	inlayOutline := outlineBmp.Mesh().SmoothSq(50).Solid()
	solid := model3d.CheckedFuncSolid(
		model3d.Origin,
		model3d.XYZ(float64(outlineBmp.Width), float64(outlineBmp.Height), Thickness),
		func(c model3d.Coord3D) bool {
			// TODO: round the corners so that nothing is contained in them.
			if inlayOutline.Contains(c.XY()) {
				return c.Z < Thickness-Inlay
			} else {
				return c.Z < Thickness
			}
		},
	)
	outsetInlayOutline := outsetSolid(outlineBmp)
	colorFn := toolbox3d.CoordColorFunc(func(c model3d.Coord3D) render3d.Color {
		if math.Abs(c.Z-(Thickness-Inlay)) < 0.1 && outsetInlayOutline.Contains(c.XY()) {
			if heartsOnly.Contains(c.XY()) {
				return render3d.NewColorRGB(1, 0, 0)
			} else {
				return render3d.NewColor(0)
			}
		}
		return render3d.NewColor(1)
	})

	xf := &model3d.VecScale{Scale: model3d.XYZ(XYScale, XYScale, 1)}
	xfColorFn := colorFn.Transform(xf)
	xfSolid := model3d.TransformSolid(xf, solid)

	log.Println("dual contouring...")
	mesh := model3d.DualContour(xfSolid, 0.1, true, false)
	log.Println("eliminating triangles...")
	mesh = mesh.EliminateCoplanarFiltered(1e-8, xfColorFn.ChangeFilterFunc(mesh, 0.1))
	log.Println("saving with colors...")
	essentials.Must(mesh.SaveVertexColorOBJ("heart_the_lover.obj", xfColorFn.SRGB))
}

// Avoid wrong color at/near edges using an outset.
func outsetSolid(bmp *model2d.Bitmap) model2d.Solid {
	return model2d.NewColliderSolidInset(
		model2d.MeshToCollider(bmp.Mesh().SmoothSq(20)),
		-2,
	)
}
