package geo

type Feature struct {
	GeometryType string
	Coordinates  interface{}
	Properties   map[string]interface{}
}

type FeatuerCollection struct {
	Features []Feature
}

// ReadShapefile reads geometry from path (.shp) and attributes from
// its sibling .dbf, paired by record order
func ReadShapefile(path string) (FeatuerCollection, error) {
	panic("TODO")
}

// ReprojectToWGS84 reads the source CRS from the sibling .prj file and
// reprojects every coordinate to EPSG:4326. No-op if already WGS84
func ReprojectToWGS84(fc FeatuerCollection, prjPath string) (FeatuerCollection, error) {
	panic("TODO")
}

// ToGeoJSON marshals fc as a GeoJSON FeatureCollection.
func ToGeoJSON(fc FeatuerCollection) ([]byte, error) {
	panic("TODO")
}
