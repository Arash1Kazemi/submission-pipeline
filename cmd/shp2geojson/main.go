package shp2geojson

func main() {
	// TODO:
	// 1. flag.String for input .shp path + output .geojson path
	// 2. geo.ReadShapefile, then geo.ReprojectToWGS84 if a .prj sidecar exists
	//    (import "wikipg/internal/geo")
	// 3. geo.ToGeoJSON, write to output path
}
