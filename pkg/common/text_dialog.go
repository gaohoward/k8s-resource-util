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
	th             *material.Theme
}

func NewTextDialog(th *material.Theme, title string, subTitle string, content string, onClose func()) *TextDialog {
	td := &TextDialog{
		title:    title,
		subTitle: subTitle,
		onClose:  onClose,
		th:       th,
	}
	td.editor = NewReadOnlyEditor(th, "content", 14, nil, false)
	td.editor.SetText(&content, nil)
	return td
}

func (td *TextDialog) Layout(
	gtx layout.Context) layout.Dimensions {

	children := make([]layout.FlexChild, 5)

	// 1 title
	children[0] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		titleLb := material.Label(td.th, unit.Sp(24), td.title)
		titleLb.Font.Weight = font.Bold
		titleLb.Color = COLOR.Blue

		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
			layout.Rigid(titleLb.Layout),
		)
	})

	biggerOne := max(td.title, td.subTitle)

	size := GetAboutWidth(gtx, td.th, biggerOne)

	// 2 horizontal divider
	children[1] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		if gtx.Constraints.Min.X == 0 {
			gtx.Constraints.Min.X = size.Size.X
		}
		div := component.Divider(td.th)
		div.Fill = td.th.Palette.ContrastBg
		div.Top = unit.Dp(4)
		div.Bottom = unit.Dp(4)

		return div.Layout(gtx)
	})

	// 3 subtitle
	children[2] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return material.Body1(td.th, td.subTitle).Layout(gtx)
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
				return layout.Center.Layout(gtx, material.Button(td.th, &td.closeClickable, "Close").Layout)
			})
		})
	})

	return layout.UniformInset(unit.Dp(30)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {

		return component.Surface(td.th).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(20)).Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						children...,
					)
				})
		})
	})
}
