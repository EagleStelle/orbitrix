package appicon

import "fmt"

// Android returns a VectorDrawable using the same paths as the web logo.
func Android() []byte {
	xml := `<?xml version="1.0" encoding="utf-8"?>
<!-- Generated from internal/appicon; update with go test ./cmd/webdist -run TestDocsLogos -update. -->
<vector xmlns:android="http://schemas.android.com/apk/res/android"
    android:width="108dp" android:height="108dp"
    android:viewportWidth="192" android:viewportHeight="192">
    <group android:translateX="96" android:translateY="96">
`
	for _, deg := range []int{45, 135, 225, 315} {
		xml += fmt.Sprintf("        <group android:rotation=\"%d\">\n            <path android:fillColor=\"#171615\" android:pathData=\"%s\" />\n        </group>\n", deg, cornerPath())
	}
	for _, p := range [][3]float64{{0, 0, orbR}, {sideD, 0, sideR}, {-sideD, 0, sideR}, {0, sideD, sideR}, {0, -sideD, sideR}} {
		x, y, r := p[0], p[1], p[2]
		xml += fmt.Sprintf("        <path android:fillColor=\"#D55E00\" android:pathData=\"M%g %gA%g %g 0 1 0 %g %gA%g %g 0 1 0 %g %gZ\" />\n", x-r, y, r, r, x+r, y, r, r, x-r, y)
	}
	return []byte(xml + "    </group>\n</vector>\n")
}
