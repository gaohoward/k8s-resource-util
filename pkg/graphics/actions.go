package graphics

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
)

var statusCheckboxValue = widget.Bool{}
var showPanelCheckboxValue = widget.Bool{}

func ShowPanel() bool {
	return showPanelCheckboxValue.Value
}

func ShowFullCr() bool {
	return statusCheckboxValue.Value
}

func GetActions(th *material.Theme) ([]component.AppBarAction, []component.OverflowAction) {
	var actions = []component.AppBarAction{
		{
			OverflowAction: component.OverflowAction{
				Name: "Show panel",
				Tag:  &showPanelCheckboxValue,
			},
			Layout: func(gtx layout.Context, bg, fg color.NRGBA) layout.Dimensions {
				btn := material.CheckBox(th, &showPanelCheckboxValue, "Show Panel")
				btn.Color = fg
				btn.IconColor = fg
				return btn.Layout(gtx)
			},
		},
	}
	return actions, nil
}
