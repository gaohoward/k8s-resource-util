package common

import "gioui.org/widget/material"

// one per top level app window
var theme *material.Theme

func GetTheme() *material.Theme {
	if theme == nil {
		theme = material.NewTheme()
	}
	return theme
}
