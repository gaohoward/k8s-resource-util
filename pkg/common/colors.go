package common

import "image/color"

var COLOR = &predefined_colors{}

type predefined_colors struct {
}

func (c *predefined_colors) White() color.NRGBA {
	return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
}

func (c *predefined_colors) Red() color.NRGBA {
	return color.NRGBA{R: 255, G: 0, B: 0, A: 255}
}

func (c *predefined_colors) Green() color.NRGBA {
	return color.NRGBA{R: 0, G: 255, B: 0, A: 255}
}

func (c *predefined_colors) DarkGreen() color.NRGBA {
	return color.NRGBA{R: 0, G: 200, B: 0, A: 255}
}

func (c *predefined_colors) Blue() color.NRGBA {
	return color.NRGBA{R: 0, G: 0, B: 255, A: 255}
}
func (c *predefined_colors) Yellow() color.NRGBA {
	return color.NRGBA{R: 255, G: 255, B: 0, A: 255}
}

func (c *predefined_colors) Cyan() color.NRGBA {
	return color.NRGBA{R: 0, G: 255, B: 255, A: 255}
}

func (c *predefined_colors) Magenta() color.NRGBA {
	return color.NRGBA{R: 255, G: 0, B: 255, A: 255}
}

func (c *predefined_colors) Black() color.NRGBA {
	return color.NRGBA{R: 0, G: 0, B: 0, A: 255}
}

func (c *predefined_colors) Gray() color.NRGBA {
	return color.NRGBA{R: 128, G: 128, B: 128, A: 255}
}
