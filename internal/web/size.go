package web

import (
	"strconv"
	"strings"
)

// The card size slider on a desk: the tile width in pixels. On a phone the
// slider sets the number of columns instead (search.js). These constants are
// the only place of the range: the page gives them to the slider, the
// script, and the --tile default.
const (
	SizeMin     = 120
	SizeMax     = 300
	SizeStep    = 20
	SizeDefault = 180
)

// SizeData is the desk range for the script: "min,max,step,default".
func SizeData() string {
	return strings.Join([]string{
		strconv.Itoa(SizeMin), strconv.Itoa(SizeMax),
		strconv.Itoa(SizeStep), strconv.Itoa(SizeDefault),
	}, ",")
}

// TileStyle sets the default tile width before the script runs.
func TileStyle() string {
	return "--tile: " + strconv.Itoa(SizeDefault) + "px"
}
