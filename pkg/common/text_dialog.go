package common

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
)

type TextDialog struct {
	title          string
	subTitle       string
	editor         *ReadOnlyEditor
	closeClickable widget.Clickable
	onClose        func()
	beforeLayout   func()
}

func NewTextDialog(title string, subTitle string, content string, onClose func(), beforeLayout func()) *TextDialog {
	td := &TextDialog{
		title:        title,
		subTitle:     subTitle,
		onClose:      onClose,
		beforeLayout: beforeLayout,
	}
	td.editor = NewReadOnlyEditor("content", 14, nil, nil, false)
	td.editor.SetText(&content, nil)
	return td
}

func (td *TextDialog) Layout(
	gtx layout.Context) layout.Dimensions {

	if td.beforeLayout != nil {
		td.beforeLayout()
	}

	children := make([]layout.FlexChild, 5)

	th := GetTheme()

	// 1 title
	children[0] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		titleLb := material.Label(th, unit.Sp(24), td.title)
		titleLb.Font.Weight = font.Bold
		titleLb.Color = COLOR.Blue

		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
			layout.Rigid(titleLb.Layout),
		)
	})

	biggerOne := max(td.title, td.subTitle)

	size := GetAboutWidth(gtx, biggerOne)

	// 2 horizontal divider
	children[1] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		if gtx.Constraints.Min.X == 0 {
			gtx.Constraints.Min.X = size.Size.X
		}
		div := component.Divider(th)
		div.Fill = th.Palette.ContrastBg
		div.Top = unit.Dp(4)
		div.Bottom = unit.Dp(4)

		return div.Layout(gtx)
	})

	// 3 subtitle
	children[2] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return material.Body1(th, td.subTitle).Layout(gtx)
	})

	// 4 content
	children[3] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return td.editor.Layout(gtx)
	})

	// 5 close button
	children[4] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		if td.closeClickable.Clicked(gtx) {
			td.onClose()
		}

		return layout.Inset{Top: unit.Dp(20), Bottom: unit.Dp(0)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, material.Button(th, &td.closeClickable, "Close").Layout)
			})
		})
	})

	return layout.UniformInset(unit.Dp(30)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {

		return component.Surface(th).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(20)).Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						children...,
					)
				})
		})
	})
}
