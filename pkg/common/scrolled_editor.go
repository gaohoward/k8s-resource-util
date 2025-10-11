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
	content  []*Liner
	textSize int
	text     *string

	menuState       component.MenuState
	menuContextArea component.ContextArea
	customActionMap map[string]MenuAction
	selectedLines   []*Liner
}

// PasteContent implements ClipboardHandler.
func (se *ReadOnlyEditor) PasteContent() *string {
	builder := strings.Builder{}
	tot := len(se.selectedLines)
	for i, line := range se.selectedLines {
		builder.WriteString(line.line.Text)
		if i < tot-1 {
			builder.WriteString("\n")
		}
	}
	content := builder.String()
	return &content
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
	GetMenuOption(th *material.Theme) func(gtx layout.Context) layout.Dimensions
	GetClickable() *widget.Clickable
	Execute(gtx layout.Context, se *ReadOnlyEditor) error
}

type EditorMenuBase struct {
	Name string
	btn  widget.Clickable
	icon *widget.Icon
}

func (emb *EditorMenuBase) GetName() string {
	return emb.Name
}

func (emb *EditorMenuBase) GetClickable() *widget.Clickable {
	return &emb.btn
}

func (emb *EditorMenuBase) GetMenuOption(th *material.Theme) func(gtx layout.Context) layout.Dimensions {
	return func(gtx layout.Context) layout.Dimensions {
		return ItemFunc(th, gtx, &emb.btn, emb.Name, emb.icon)
	}
}

type CopyMenuAction struct {
	EditorMenuBase
}

func (cma *CopyMenuAction) Execute(gtx layout.Context, editor *ReadOnlyEditor) error {
	gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(editor.GetText()))})
	return nil
}

type SaveMenuAction struct {
	EditorMenuBase
}

func (sma *SaveMenuAction) Execute(gtx layout.Context, editor *ReadOnlyEditor) error {
	go func() {
		writer, err := GetExplorer().CreateFile("unnamed")
		if err != nil {
			logger.Info("failed to save file", zap.Error(err))
			return
		}
		defer writer.Close()

		for _, liner := range editor.content {
			writer.Write([]byte(liner.line.Text + "\n"))
		}
	}()

	return nil
}

type CopySelectionMenuAction struct {
	EditorMenuBase
}

func (sma *CopySelectionMenuAction) Execute(gtx layout.Context, editor *ReadOnlyEditor) error {
	Copy(editor)
	return nil
}

func NewCopyMenuAction() *CopyMenuAction {
	copyAct := &CopyMenuAction{
		EditorMenuBase: EditorMenuBase{
			Name: "Copy All",
			icon: graphics.CopyIcon,
		},
	}
	return copyAct
}

func NewSaveMenuAction() *SaveMenuAction {
	saveAct := &SaveMenuAction{
		EditorMenuBase: EditorMenuBase{
			Name: "Save",
			icon: graphics.SaveIcon,
		},
	}
	return saveAct
}

func NewCopySelectionMenuAction() *CopySelectionMenuAction {
	copySelAct := &CopySelectionMenuAction{
		EditorMenuBase: EditorMenuBase{
			Name: "Copy Selection",
			icon: graphics.CopyIcon,
		},
	}
	return copySelAct
}

func NewReadOnlyEditor(th *material.Theme, hint string, textSize int, actions []MenuAction) *ReadOnlyEditor {
	se := &ReadOnlyEditor{
		th:              th,
		textSize:        textSize,
		customActionMap: make(map[string]MenuAction),
		selectedLines:   make([]*Liner, 0),
	}

	se.list.Axis = layout.Vertical

	menuOptions := make([]func(gtx layout.Context) layout.Dimensions, 0)

	allActs := make([]MenuAction, 0)

	copyAct := NewCopyMenuAction()
	allActs = append(allActs, copyAct)

	saveAct := NewSaveMenuAction()
	allActs = append(allActs, saveAct)

	copySelAct := NewCopySelectionMenuAction()
	allActs = append(allActs, copySelAct)

	allActs = append(allActs, actions...)

	for _, action := range allActs {
		se.customActionMap[action.GetName()] = action
		menuOptions = append(menuOptions, action.GetMenuOption(th))
	}

	se.menuState = component.MenuState{
		Options: menuOptions,
	}
	return se
}

type Liner struct {
	content    *string
	line       material.LabelStyle
	lineNumber material.LabelStyle
	clickable  widget.Clickable
	isSelected bool
}

func (l *Liner) Clicked() bool {
	l.isSelected = !l.isSelected
	return l.isSelected
}

func (l *Liner) Layout(gtx layout.Context, lineWidth int, index int) layout.Dimensions {

	if l.isSelected {
		l.lineNumber.Color = COLOR.Black()
		l.line.Color = COLOR.Blue()
	} else {
		l.lineNumber.Color = COLOR.Gray()
		l.line.Color = COLOR.Black()
	}
	l.lineNumber.Text = (fmt.Sprintf("%d", index+1))

	return material.Clickable(gtx, &l.clickable, func(gtx layout.Context) layout.Dimensions {

		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {

				gtx.Constraints.Min.X = lineWidth

				return l.lineNumber.Layout(gtx)
			}),
			layout.Flexed(1.0, l.line.Layout),
		)

	})
}

func (se *ReadOnlyEditor) NewLiner(content *string) *Liner {

	// only show up to 1024 chars
	var displayContent string
	if len(*content) > 1024 {
		displayContent = (*content)[:1024]
	} else {
		displayContent = *content
	}
	lineNumber := material.Label(se.th, unit.Sp(se.textSize), fmt.Sprintf("%d", 10))
	lineNumber.TextSize = unit.Sp(se.textSize - 3)
	lineNumber.LineHeight = unit.Sp(se.textSize - 2)
	lineNumber.Color = COLOR.Gray()
	lineNumber.Font.Typeface = "monospace"

	line := material.Label(se.th, unit.Sp(se.textSize), displayContent)
	line.TextSize = unit.Sp(se.textSize)
	line.LineHeight = unit.Sp(se.textSize)
	line.MaxLines = 3
	line.Font.Typeface = "monospace"

	return &Liner{
		content:    content,
		line:       line,
		lineNumber: lineNumber,
	}
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

			lineNumberWidth := -1
			return listStyle.Layout(gtx, tot, func(gtx layout.Context, index int) layout.Dimensions {

				liner := se.content[index]

				if liner.clickable.Clicked(gtx) {
					liner.Clicked()
					se.updateSelections(liner)
				}

				if lineNumberWidth == -1 {
					liner.lineNumber.Text = fmt.Sprintf("%d", len(se.content)*10)
					macro := op.Record(gtx.Ops)
					numSize := liner.lineNumber.Layout(gtx)
					macro.Stop()
					lineNumberWidth = numSize.Size.X
				}

				return liner.Layout(gtx, lineNumberWidth, index)

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

func (se *ReadOnlyEditor) updateSelections(liner *Liner) {
	// suppose user won't select many lines. otherwise consider
	// using a map
	found := false
	for i, l := range se.selectedLines {
		if l == liner {
			// remove from selectedLines
			se.selectedLines = append(se.selectedLines[:i], se.selectedLines[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		se.selectedLines = append(se.selectedLines, liner)
	}
}

func (se *ReadOnlyEditor) SetText(text *string) {
	se.text = text

	scanner := bufio.NewScanner(strings.NewReader(*text))
	if len(*text) > bufio.MaxScanTokenSize {
		scanner.Buffer(make([]byte, len(*text)), len(*text))
	}
	se.content = make([]*Liner, 0)
	for scanner.Scan() {
		line := scanner.Text()
		se.content = append(se.content, se.NewLiner(&line))
	}
	if err := scanner.Err(); err != nil {
		msg := err.Error()
		se.content = append(se.content, se.NewLiner(&msg))
	}
}

func (se *ReadOnlyEditor) SetHint(text *string) {
	se.SetText(text)
}
