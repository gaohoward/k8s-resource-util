package common

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"image"
	"io"
	"strings"
	"sync"

	"gaohoward.tools/k8s/resutil/pkg/graphics"
	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Filter struct {
	lock            sync.RWMutex
	filterField     *SearchBar
	originalContent []*Liner
	previousWork    []*Liner
	editorId        string
}

type RefInfo struct {
	LineNumber  int      `yaml:"line_number"`
	InlineNote  string   `yaml:"inline_note"`
	RefFileUrls []string `yaml:"ref_file_urls"`
}

func (f *Filter) SetOriginalContent(content []*Liner) {
	f.lock.Lock()
	defer f.lock.Unlock()

	f.originalContent = content
}

func NewFilter(original []*Liner, editorId string) *Filter {
	filter := &Filter{
		filterField:     NewSearchBar(editorId),
		originalContent: original,
		previousWork:    make([]*Liner, 0),
	}

	if filter.originalContent == nil {
		filter.originalContent = make([]*Liner, 0)
	}

	return filter
}

func (f *Filter) GetFiltered() []*Liner {
	f.lock.RLock()
	defer f.lock.RUnlock()

	if f.filterField.GetText() == "" {
		return f.originalContent
	}
	return f.previousWork
}

func (f *Filter) filterContent() {
	f.lock.Lock()
	defer f.lock.Unlock()

	filterText := f.filterField.GetText()

	if filterText != "" {
		result := make([]*Liner, 0)
		count := 0
		for _, liner := range f.originalContent {
			if liner.Match(filterText, f.filterField.IsCaseSensitive()) {
				count++
				result = append(result, liner)
			}
		}
		f.previousWork = result
		SetContextBool(f.editorId, true, nil)
	}
}

func (f *Filter) Layout(gtx layout.Context) layout.Dimensions {

	changed := f.filterField.Changed(gtx)

	if changed {
		go f.filterContent()
	}

	return f.filterField.Layout(gtx)
}

type EditorEventListener interface {
	ExtraLinkClicked(link *ExtraLink)
}

// as editors doesn't have scroll bar support
// we created this 'editor' using a list
type ReadOnlyEditor struct {
	id            string
	list          widget.List
	originContent []*Liner
	textSize      int
	text          *string

	menuState       component.MenuState
	menuContextArea component.ContextArea
	customActionMap map[string]MenuAction
	selectedLines   []*Liner

	filterOn bool
	filter   *Filter

	eventListener EditorEventListener
}

func (se *ReadOnlyEditor) GetId() string {
	return se.id
}

func (se *ReadOnlyEditor) RegisterEditorListener(listener EditorEventListener) error {
	if se.eventListener != nil {
		return fmt.Errorf("listener already set")
	}
	se.eventListener = listener
	return nil
}

func (se *ReadOnlyEditor) ExtraLinkClicked(link *ExtraLink) {
	if se.eventListener != nil {
		se.eventListener.ExtraLinkClicked(link)
	}
}

func (se *ReadOnlyEditor) Clear() {
	empty := ""
	se.SetText(&empty, nil)
}

func (se *ReadOnlyEditor) GetSelectedLines() []*Liner {
	return se.selectedLines
}

// PasteContent implements ClipboardHandler.
func (se *ReadOnlyEditor) PasteContent() *string {
	builder := strings.Builder{}
	tot := len(se.selectedLines)
	for i, line := range se.selectedLines {
		builder.WriteString(*line.content)
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
	GetMenuOption() func(gtx layout.Context) layout.Dimensions
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

func (emb *EditorMenuBase) GetMenuOption() func(gtx layout.Context) layout.Dimensions {
	return func(gtx layout.Context) layout.Dimensions {
		return ItemFunc(gtx, &emb.btn, emb.Name, emb.icon)
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

		for _, liner := range editor.originContent {
			writer.Write([]byte(liner.lineLabel.Text + "\n"))
		}
	}()

	return nil
}

type SaveSelectionMenuAction struct {
	EditorMenuBase
}

type ClearSelectionMenuAction struct {
	EditorMenuBase
}

func (sma *ClearSelectionMenuAction) Execute(gtx layout.Context, editor *ReadOnlyEditor) error {
	for _, liner := range editor.selectedLines {
		liner.isSelected = false
	}
	editor.selectedLines = make([]*Liner, 0)
	return nil
}

func (sma *SaveSelectionMenuAction) Execute(gtx layout.Context, editor *ReadOnlyEditor) error {
	go func() {
		writer, err := GetExplorer().CreateFile("selection_unnamed")
		if err != nil {
			logger.Info("failed to save file", zap.Error(err))
			return
		}
		defer writer.Close()

		for _, liner := range editor.selectedLines {
			writer.Write([]byte(*liner.content + "\n"))
		}
	}()

	return nil
}

type CopySelectionMenuAction struct {
	EditorMenuBase
}

func (sma *CopySelectionMenuAction) Execute(gtx layout.Context, editor *ReadOnlyEditor) error {
	Copy(editor)
	gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(*editor.PasteContent()))})
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

func NewSaveSelectionMenuAction() *SaveSelectionMenuAction {
	saveSelAct := &SaveSelectionMenuAction{
		EditorMenuBase: EditorMenuBase{
			Name: "Save Selection",
			icon: graphics.SaveIcon,
		},
	}
	return saveSelAct
}

func NewClearSelectionMenuAction() *ClearSelectionMenuAction {
	clearSelAct := &ClearSelectionMenuAction{
		EditorMenuBase: EditorMenuBase{
			Name: "Clear Selection",
			icon: graphics.ClearIcon,
		},
	}
	return clearSelAct
}

func NewReadOnlyEditor(hint string, textSize int, actions []MenuAction, useFilter bool) *ReadOnlyEditor {
	se := &ReadOnlyEditor{
		id:              uuid.New().String(),
		textSize:        textSize,
		customActionMap: make(map[string]MenuAction),
		selectedLines:   make([]*Liner, 0),
		filterOn:        useFilter,
	}

	if useFilter {
		se.filter = NewFilter(nil, se.id)
		RegisterContext(se.id, false, true)
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

	saveSelAct := NewSaveSelectionMenuAction()
	allActs = append(allActs, saveSelAct)

	clearSelAct := NewClearSelectionMenuAction()
	allActs = append(allActs, clearSelAct)

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

type LineNote struct {
	Note          string
	noteClickable widget.Clickable
	noteLabel     material.LabelStyle
}

func NewLineNote(note string, se *ReadOnlyEditor) *LineNote {
	ln := &LineNote{
		Note: note,
	}

	ln.noteLabel = material.Label(GetTheme(), unit.Sp(se.textSize), "[note]")
	ln.noteLabel.TextSize = unit.Sp(se.textSize)
	ln.noteLabel.LineHeight = unit.Sp(se.textSize)
	ln.noteLabel.MaxLines = 1
	ln.noteLabel.Color = COLOR.Red
	ln.noteLabel.Font.Typeface = "monospace"

	return ln
}

type ExtraLink struct {
	Id            int
	SourceEditor  *ReadOnlyEditor
	Link          string
	linkClickable widget.Clickable
	linkLabel     *material.LabelStyle
}

func (el *ExtraLink) Layout(gtx layout.Context) layout.Dimensions {
	return el.linkLabel.Layout(gtx)
}

type Liner struct {
	content        *string
	extraContent   *string
	extraClickable widget.Clickable
	extraLabel     material.LabelStyle
	extraDialog    *TextDialog
	showExtra      bool

	note       *LineNote
	showNote   bool
	noteDialog *TextDialog

	extraLinks []*ExtraLink
	linkList   widget.List

	lineLabel               material.LabelStyle
	lineNumberLabel         material.LabelStyle
	originalLineNumberLabel material.LabelStyle
	clickable               widget.Clickable
	isSelected              bool
	originalLineIndex       int
}

func (l *Liner) GetLinks() []*ExtraLink {
	return l.extraLinks
}

func (l *Liner) RemoveNote() *string {
	oldNote := l.GetNote()
	if l.note != nil {
		l.note = nil
	}
	return oldNote
}

func (l *Liner) AddNote(content string, se *ReadOnlyEditor) {
	if strings.TrimSpace(content) == "" {
		return
	}
	if l.note == nil {
		l.note = NewLineNote(content, se)
		l.noteDialog.editor.SetText(&content, nil)
	} else {
		l.note.Note = content
		l.noteDialog.editor.SetText(&content, nil)
	}
}

func (l *Liner) AddRefLink(link string, editor *ReadOnlyEditor) {
	id := len(l.extraLinks)
	exLink := NewExtraLink(id, editor, link, l.lineLabel.TextSize)
	l.extraLinks = append(l.extraLinks, exLink)
}

func (l *Liner) RemoveRefLinks(links []*Choice) {
	newLinks := make([]*ExtraLink, 0)
	for _, el := range l.extraLinks {
		match := false
		for _, toRm := range links {
			if ref, ok := toRm.Value.(*ExtraLink); ok {
				if el.Id == ref.Id {
					match = true
					break
				}
			}
		}
		if !match {
			newLinks = append(newLinks, el)
		}
	}
	l.extraLinks = newLinks
}

func (l *Liner) GetLineNumber() int {
	return l.originalLineIndex
}

func (l *Liner) GetNote() *string {
	if l.note != nil {
		return &l.note.Note
	}
	return nil
}

func (l *Liner) Match(filterText string, caseSensitive bool) bool {
	if caseSensitive {
		return strings.Contains(*l.content, filterText)
	}
	return strings.Contains(strings.ToLower(*l.content), strings.ToLower(filterText))
}

func (l *Liner) Clicked() bool {
	l.isSelected = !l.isSelected
	return l.isSelected
}

func (l *Liner) Layout(gtx layout.Context, lineWidth int, index int) layout.Dimensions {

	if l.isSelected {
		l.lineNumberLabel.Color = COLOR.Black
		l.lineLabel.Color = COLOR.Blue
	} else {
		l.lineNumberLabel.Color = COLOR.LightGray
		l.lineLabel.Color = COLOR.Black
	}
	l.lineNumberLabel.Text = (fmt.Sprintf("%d", index+1))

	if l.extraContent != nil {
		if l.extraClickable.Clicked(gtx) {
			l.showExtra = true
		}
		if l.showExtra {
			return l.extraDialog.Layout(gtx)
		}
	}

	if l.note != nil {
		if l.note.noteClickable.Clicked(gtx) {
			l.showNote = true
		}

		if l.showNote {
			return l.noteDialog.Layout(gtx)
		}
	}

	if l.originalLineIndex != index {
		//filtered
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				return material.Clickable(gtx, &l.clickable, func(gtx layout.Context) layout.Dimensions {

					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.X = lineWidth
							return l.lineNumberLabel.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return l.originalLineNumberLabel.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if len(l.extraLinks) > 0 {
								return l.linkList.Layout(gtx, len(l.extraLinks), func(gtx layout.Context, index int) layout.Dimensions {
									lk := l.extraLinks[index]
									if lk.linkClickable.Clicked(gtx) {
										lk.SourceEditor.ExtraLinkClicked(lk)
									}

									return material.Clickable(gtx, &lk.linkClickable, lk.Layout)
								})
							}
							return layout.Dimensions{}
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return l.lineLabel.Layout(gtx)
						}),
					)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Clickable(gtx, &l.extraClickable, func(gtx layout.Context) layout.Dimensions {
					if l.extraContent != nil {
						return l.extraLabel.Layout(gtx)
					}
					return layout.Dimensions{}
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if l.note != nil {
					return material.Clickable(gtx, &l.note.noteClickable, func(gtx layout.Context) layout.Dimensions {
						return l.note.noteLabel.Layout(gtx)
					})
				}
				return layout.Dimensions{}
			}),
		)
	}

	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Clickable(gtx, &l.clickable, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								gtx.Constraints.Min.X = lineWidth
								return l.lineNumberLabel.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if len(l.extraLinks) > 0 {
									return l.linkList.Layout(gtx, len(l.extraLinks), func(gtx layout.Context, index int) layout.Dimensions {
										lk := l.extraLinks[index]
										if lk.linkClickable.Clicked(gtx) {
											lk.SourceEditor.ExtraLinkClicked(lk)
										}

										return material.Clickable(gtx, &lk.linkClickable, lk.Layout)
									})
								}
								return layout.Dimensions{}
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return l.lineLabel.Layout(gtx)
							}),
						)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Clickable(gtx, &l.extraClickable, func(gtx layout.Context) layout.Dimensions {
						if l.extraContent != nil {
							return l.extraLabel.Layout(gtx)
						}
						return layout.Dimensions{}
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if l.note != nil {
						return material.Clickable(gtx, &l.note.noteClickable, func(gtx layout.Context) layout.Dimensions {
							return l.note.noteLabel.Layout(gtx)
						})
					}
					return layout.Dimensions{}
				}),
			)
		}),
	)
}

func (se *ReadOnlyEditor) NewLiner(content *string, index int, extraContent *string, note *string, links []string) *Liner {

	th := GetTheme()
	// only show up to 1024 chars
	var displayContent string
	if len(*content) > 1024 {
		displayContent = (*content)[:1024]
	} else {
		displayContent = *content
	}

	lineNumber := material.Label(th, unit.Sp(se.textSize), fmt.Sprintf("%d", 10))
	lineNumber.TextSize = unit.Sp(se.textSize - 3)
	lineNumber.LineHeight = unit.Sp(se.textSize - 2)
	lineNumber.Color = COLOR.LightGray
	lineNumber.Font.Typeface = "monospace"

	originalLineNumber := material.Label(th, unit.Sp(se.textSize), fmt.Sprintf("%d", 10))
	originalLineNumber.TextSize = unit.Sp(se.textSize - 3)
	originalLineNumber.LineHeight = unit.Sp(se.textSize - 2)
	originalLineNumber.Color = COLOR.Blue
	originalLineNumber.Font.Typeface = "monospace"
	originalLineNumber.Text = fmt.Sprintf("(%d)", index+1)

	line := material.Label(th, unit.Sp(se.textSize), displayContent)
	line.TextSize = unit.Sp(se.textSize)
	line.LineHeight = unit.Sp(se.textSize)
	line.MaxLines = 3
	line.Font.Typeface = "monospace"

	extraLabel := material.Label(th, unit.Sp(se.textSize), "...")
	extraLabel.TextSize = unit.Sp(se.textSize)
	extraLabel.LineHeight = unit.Sp(se.textSize)
	extraLabel.MaxLines = 1
	extraLabel.Color = COLOR.Red
	extraLabel.Font.Typeface = "monospace"

	l := &Liner{
		content:                 content,
		extraContent:            extraContent,
		lineLabel:               line,
		lineNumberLabel:         lineNumber,
		originalLineNumberLabel: originalLineNumber,
		originalLineIndex:       index,
		extraLabel:              extraLabel,
	}

	if l.extraContent != nil {
		l.extraDialog = NewTextDialog("content", "", *l.extraContent, func() {
			l.showExtra = false
		})
	}

	if note != nil {
		l.note = NewLineNote(*note, se)
		l.noteDialog = NewTextDialog("note", "", *note, func() {
			l.showNote = false
		})
	} else {
		l.noteDialog = NewTextDialog("note", "", "", func() {
			l.showNote = false
		})
	}

	l.extraLinks = make([]*ExtraLink, 0)
	for i, lk := range links {
		el := NewExtraLink(i, se, lk, unit.Sp(se.textSize))
		l.extraLinks = append(l.extraLinks, el)
	}

	return l
}

func NewExtraLink(id int, editor *ReadOnlyEditor, link string, textSize unit.Sp) *ExtraLink {
	el := &ExtraLink{
		Id:           id,
		SourceEditor: editor,
		Link:         link,
	}

	linkLabel := material.Label(GetTheme(), unit.Sp(textSize), link)
	linkLabel.LineHeight = unit.Sp(textSize)
	linkLabel.MaxLines = 1
	linkLabel.Color = COLOR.Red
	linkLabel.Font.Typeface = "monospace"
	linkLabel.Font.Weight = font.Bold
	linkLabel.Text = fmt.Sprintf("[%d]", id)

	el.linkLabel = &linkLabel

	return el
}

func (se *ReadOnlyEditor) Layout(gtx layout.Context) layout.Dimensions {

	for _, v := range se.customActionMap {
		if v.GetClickable().Clicked(gtx) {
			v.Execute(gtx, se)
		}
	}

	listStyle := material.List(GetTheme(), &se.list)
	content := se.GetContent()
	tot := len(content)

	filterPart := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return se.LayoutFilter(gtx)
	})

	contentPart := layout.Rigid(func(gtx layout.Context) layout.Dimensions {

		lineNumberWidth := -1
		return listStyle.Layout(gtx, tot, func(gtx layout.Context, index int) layout.Dimensions {

			liner := content[index]

			if liner.clickable.Clicked(gtx) {
				liner.Clicked()
				se.updateSelections(liner)
			}

			if lineNumberWidth == -1 {
				liner.lineNumberLabel.Text = fmt.Sprintf("%d", len(content)*10)
				macro := op.Record(gtx.Ops)
				numSize := liner.lineNumberLabel.Layout(gtx)
				macro.Stop()
				lineNumberWidth = numSize.Size.X
			}

			return liner.Layout(gtx, lineNumberWidth, index)

		})
	})

	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {

			if len(se.originContent) == 0 {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					contentPart,
				)
			}

			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				filterPart,
				contentPart,
			)
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return se.menuContextArea.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Point{}
				return component.Menu(GetTheme(), &se.menuState).Layout(gtx)
			})
		}),
	)
}

func (se *ReadOnlyEditor) LayoutFilter(gtx layout.Context) layout.Dimensions {
	if se.filterOn {
		return se.filter.Layout(gtx)
	}
	return layout.Dimensions{}
}

func (se *ReadOnlyEditor) GetContent() []*Liner {
	if se.filterOn {
		return se.filter.GetFiltered()
	}
	return se.originContent
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

const begin_token = "[$X$["
const end_token = "]$X$]"

func MakeExtraFragment(original string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(original))
	return begin_token + encoded + end_token
}

const begin_link_token = "[$L$["
const end_link_token = "]$L$]"

func MakeLinkFragment(link string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(link))
	return begin_link_token + encoded + end_link_token
}

// extra content goes to a button popup to make each line displayed
// nicely while can carrying extra infomation
// to include extra messages in a line
// wrap it in an begin_token / end_token pair
// currently only support one extra content at the end of line
// so the original line must be of the following pattern
// <message><begin_token><extra content base64><end_token> (so end_token is optional)
// using base64 can bypass the newlines processing in the line
func ParseLineContent(line *string, lineNumber int, extras map[int]*RefInfo) (*string, *string, []string, *string) {
	var l, extra, note *string = nil, nil, nil
	links := make([]string, 0)

	l = line

	beginIdx := strings.LastIndex(*line, begin_token)
	if beginIdx != -1 {
		line1 := (*line)[:beginIdx]
		l = &line1

		extra1 := (*line)[beginIdx+len(begin_token):]
		endIdx := strings.LastIndex(extra1, end_token)
		if endIdx != -1 {
			extra1 = extra1[:endIdx]
		}
		if extra1 != "" {
			decoded, _ := base64.StdEncoding.DecodeString(extra1)
			extra1 = string(decoded)
		}
		extra = &extra1
	}

	beginLinkIdx := strings.LastIndex(*line, begin_link_token)
	if beginLinkIdx != -1 {
		if extra == nil {
			lineWithoutLinks := (*line)[:beginLinkIdx]
			l = &lineWithoutLinks
		}

		link1 := (*line)[beginLinkIdx+len(begin_link_token):]
		endLinkIdx := strings.LastIndex(link1, end_link_token)
		if endLinkIdx != -1 {
			link1 = link1[:endLinkIdx]
		}

		if link1 != "" {
			parts := strings.SplitSeq(link1, " ")
			for part := range parts {
				decoded, _ := base64.StdEncoding.DecodeString(part)
				part = string(decoded)
				links = append(links, part)
			}
		}
	}

	if extras != nil {
		if info, ok := extras[lineNumber]; ok {
			if info.InlineNote != "" {
				note = &info.InlineNote
			}
			if len(info.RefFileUrls) > 0 {
				links = append(links, info.RefFileUrls...)
			}
		}
	}

	return l, extra, links, note
}

func (se *ReadOnlyEditor) SetText(text *string, extras map[int]*RefInfo) {
	se.text = text
	se.selectedLines = make([]*Liner, 0)

	scanner := bufio.NewScanner(strings.NewReader(*text))
	if len(*text) > bufio.MaxScanTokenSize {
		scanner.Buffer(make([]byte, len(*text)), len(*text))
	}
	se.originContent = make([]*Liner, 0)
	for scanner.Scan() {
		line := scanner.Text()
		lineNumber := len(se.originContent)
		ln, extra, links, note := ParseLineContent(&line, lineNumber, extras)
		liner := se.NewLiner(ln, lineNumber, extra, note, links)
		se.originContent = append(se.originContent, liner)
	}
	if err := scanner.Err(); err != nil {
		msg := err.Error()
		se.originContent = append(se.originContent, se.NewLiner(&msg, len(se.originContent), nil, nil, nil))
	}
	if se.filterOn {
		se.filter.SetOriginalContent(se.originContent)
	}
}

func (se *ReadOnlyEditor) SetHint(text *string) {
	se.SetText(text, nil)
}
