package common

import "image/color"

var COLOR = NewPredefinedColors()

type predefined_colors struct {
	White     color.NRGBA
	Red       color.NRGBA
	Green     color.NRGBA
	DarkGreen color.NRGBA
	Blue      color.NRGBA
	Yellow    color.NRGBA
	Cyan      color.NRGBA
	Magenta   color.NRGBA
	Black     color.NRGBA
	Gray      color.NRGBA
	LightGray color.NRGBA
}

func NewPredefinedColors() *predefined_colors {
	precolors := &predefined_colors{
		White:     color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		Red:       color.NRGBA{R: 255, G: 0, B: 0, A: 255},
		Green:     color.NRGBA{R: 0, G: 255, B: 0, A: 255},
		DarkGreen: color.NRGBA{R: 0, G: 200, B: 0, A: 255},
		Blue:      color.NRGBA{R: 0, G: 0, B: 255, A: 255},
		Yellow:    color.NRGBA{R: 255, G: 255, B: 0, A: 255},
		Cyan:      color.NRGBA{R: 0, G: 255, B: 255, A: 255},
		Magenta:   color.NRGBA{R: 255, G: 0, B: 255, A: 255},
		Black:     color.NRGBA{R: 0, G: 0, B: 0, A: 255},
		Gray:      color.NRGBA{R: 128, G: 128, B: 128, A: 255},
		LightGray: color.NRGBA{R: 211, G: 211, B: 211, A: 255},
	}
	return precolors
}
