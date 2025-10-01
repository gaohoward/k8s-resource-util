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
	"go.uber.org/zap"
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
	menuContextArea component.ContextArea
	customActionMap map[string]MenuAction
}

func (se *ReadOnlyEditor) GetText() string {
	if se.text != nil {
		return *se.text
	} else {
		return ""
	}
}

type MenuAction interface {
	GetName() string
	GetMenuOption() func(gtx layout.Context) layout.Dimensions
	GetClickable() *widget.Clickable
	Execute(gtx layout.Context, se *ReadOnlyEditor) error
}

type CopyMenuAction struct {
	Name     string
	copyBtn  widget.Clickable
	MenuFunc func(gtx layout.Context) layout.Dimensions
}

type SaveMenuAction struct {
	Name     string
	saveBtn  widget.Clickable
	MenuFunc func(gtx layout.Context) layout.Dimensions
}

// GetMenuOption implements MenuAction.
func (cma *CopyMenuAction) GetMenuOption() func(gtx layout.Context) layout.Dimensions {
	return cma.MenuFunc
}

// GetName implements MenuAction.
func (cma *CopyMenuAction) GetName() string {
	return cma.Name
}

func (cma *CopyMenuAction) GetClickable() *widget.Clickable {
	return &cma.copyBtn
}

func (cma *CopyMenuAction) Execute(gtx layout.Context, editor *ReadOnlyEditor) error {
	gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(editor.GetText()))})
	return nil
}

func NewCopyMenuAction(th *material.Theme) *CopyMenuAction {
	copyAct := &CopyMenuAction{
		Name: "Copy",
	}
	copyAct.MenuFunc = func(gtx layout.Context) layout.Dimensions {
		return ItemFunc(th, gtx, &copyAct.copyBtn, copyAct.Name, graphics.CopyIcon)
	}
	return copyAct
}

func (sma *SaveMenuAction) GetMenuOption() func(gtx layout.Context) layout.Dimensions {
	return sma.MenuFunc
}

// GetName implements MenuAction.
func (sma *SaveMenuAction) GetName() string {
	return sma.Name
}

func (sma *SaveMenuAction) GetClickable() *widget.Clickable {
	return &sma.saveBtn
}

func (sma *SaveMenuAction) Execute(gtx layout.Context, editor *ReadOnlyEditor) error {
	go func() {
		writer, err := GetExplorer().CreateFile("unnamed")
		if err != nil {
			logger.Info("failed to save file", zap.Error(err))
			return
		}
		defer writer.Close()

		for _, line := range editor.content {
			writer.Write([]byte(line + "\n"))
		}
	}()

	return nil
}

func NewSaveMenuAction(th *material.Theme) *SaveMenuAction {
	saveAct := &SaveMenuAction{
		Name: "Save",
	}
	saveAct.MenuFunc = func(gtx layout.Context) layout.Dimensions {
		return ItemFunc(th, gtx, &saveAct.saveBtn, saveAct.Name, graphics.SaveIcon)
	}
	return saveAct
}

func NewReadOnlyEditor(th *material.Theme, hint string, textSize int, actions []MenuAction) *ReadOnlyEditor {
	se := &ReadOnlyEditor{
		th:              th,
		textSize:        textSize,
		customActionMap: make(map[string]MenuAction),
	}

	se.list.Axis = layout.Vertical

	menuOptions := make([]func(gtx layout.Context) layout.Dimensions, 0)

	allActs := make([]MenuAction, 0)

	copyAct := NewCopyMenuAction(th)
	allActs = append(allActs, copyAct)

	saveAct := NewSaveMenuAction(th)
	allActs = append(allActs, saveAct)

	allActs = append(allActs, actions...)

	for _, action := range allActs {
		se.customActionMap[action.GetName()] = action
		menuOptions = append(menuOptions, action.GetMenuOption())
	}

	se.menuState = component.MenuState{
		Options: menuOptions,
	}
	return se
}

func (se *ReadOnlyEditor) Layout(gtx layout.Context) layout.Dimensions {

	for _, v := range se.customActionMap {
		if v.GetClickable().Clicked(gtx) {
			v.Execute(gtx, se)
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
				lineNum.TextSize = unit.Sp(se.textSize - 3)
				lineNum.LineHeight = unit.Sp(se.textSize - 2)
				lineNum.Color = COLOR.Gray()
				lineNum.Font.Typeface = "monospace"

				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
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
