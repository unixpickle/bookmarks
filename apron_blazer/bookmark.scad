img_scale = 0.075;

module black_part() {
  marching_squares(0.3)
    solid()
    outset_sdf(0.025)
    mesh_to_sdf()
    smooth_sq(20)
    scale(img_scale)
    image("black.png");
}

module grey_part() {
  marching_squares(0.5)
    solid()
    outset_sdf(0.1)
    mesh_to_sdf()
    smooth_sq(20)
    scale(img_scale)
    image("grey.png");
}

module white_part() {
  marching_squares(0.3)
    solid()
    outset_sdf(0.15)
    mesh_to_sdf()
    smooth_sq(10)
    scale(img_scale)
    image("white.png");
}

cache black_mesh() {
  difference() {
    linear_extrude(3)
      difference() {
        black_part();
        grey_part();
        white_part();
      }
  }
}

cache white_mesh() {
  linear_extrude(3)
    white_part();
}

cache grey_mesh() {
  linear_extrude(3)
    grey_part();
}

// Uncomment one at a time to create the separate meshes.

//black_mesh();
//white_mesh();
grey_mesh();
