package common

import (
	"bufio"
	"fmt"
	"image"
	"io"
	"strings"

	"gaohoward.tools/k8s/resutil/pkg/graphics"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
)

// as editors doesn't have scroll bar support
// we created this 'editor' using a list
type ReadOnlyEditor struct {
	th       *material.Theme
	list     widget.List
	content  []string
	textSize int
	text     *string

	menuState       component.MenuState
	copyBtn         widget.Clickable
	menuContextArea component.ContextArea
}

func NewReadOnlyEditor(th *material.Theme, hint string, textSize int) *ReadOnlyEditor {
	se := &ReadOnlyEditor{
		th:       th,
		textSize: textSize,
	}

	se.list.Axis = layout.Vertical

	se.menuState = component.MenuState{
		Options: []func(gtx component.C) component.D{
			func(gtx component.C) component.D {
				return ItemFunc(th, gtx, &se.copyBtn, "Copy", graphics.CopyIcon)
			},
		},
	}
	return se
}

func (se *ReadOnlyEditor) Layout(gtx layout.Context) layout.Dimensions {
	if se.copyBtn.Clicked(gtx) {
		if se.text != nil {
			gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(*se.text))})
		}
	}

	listStyle := material.List(se.th, &se.list)
	tot := len(se.content)

	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return listStyle.Layout(gtx, tot, func(gtx layout.Context, index int) layout.Dimensions {
				line := material.Label(se.th, unit.Sp(se.textSize), se.content[index])
				line.TextSize = unit.Sp(se.textSize)
				line.LineHeight = unit.Sp(se.textSize)
				line.MaxLines = 3
				line.Font.Typeface = "monospace"

				lineNum := material.Label(se.th, unit.Sp(se.textSize), fmt.Sprintf("%d", tot*10))
				lineNum.TextSize = unit.Sp(se.textSize - 4)
				lineNum.LineHeight = unit.Sp(se.textSize - 4)
				lineNum.Color = COLOR.Gray()
				lineNum.Font.Typeface = "monospace"

				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						macro := op.Record(gtx.Ops)
						numSize := lineNum.Layout(gtx)
						macro.Stop()

						lineNum.Text = (fmt.Sprintf("%d", index+1))

						gtx.Constraints.Min.X = numSize.Size.X

						return lineNum.Layout(gtx)
					}),
					layout.Flexed(1.0, line.Layout),
				)
			})
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return se.menuContextArea.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Point{}
				return component.Menu(se.th, &se.menuState).Layout(gtx)
			})
		}),
	)
}

func (se *ReadOnlyEditor) SetText(text *string) {
	se.text = text

	scanner := bufio.NewScanner(strings.NewReader(*text))
	se.content = make([]string, 0)
	for scanner.Scan() {
		se.content = append(se.content, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		se.content = append(se.content, err.Error())
	}
}

func (se *ReadOnlyEditor) SetHint(text *string) {
	se.SetText(text)
}
