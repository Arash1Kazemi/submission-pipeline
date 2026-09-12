package imagepipe

type Variant struct {
	Label  string
	Width  int
	Height int
	Path   string
	Bytes  int64
}

type Result struct {
	OriginalWidth  int
	OriginalHeight int
	Variants       []Variant
	Rejected       bool
	RejectReason   string
}

// Sizes are thumbnail widths 'px'
var Sizes = []int{150, 400, 1200}

func Process(srcPath, outDir string) (Result, error) {
	panic("TODO")
}

// Validates decodes srcPhat and checks format/dimension/size limits
func Validate(srcPath string) (rejected bool, reason string, err error) {
	panic("TODO")
}

// stripEXIF removes metadata via decode then re-encode
func makeThumbnail(srcPath, outDir string, sizes []int) ([]Variant, error) {
	panic("TODO")
}
