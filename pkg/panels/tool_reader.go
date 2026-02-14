package panels

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gaohoward.tools/k8s/resutil/pkg/common"
	"gaohoward.tools/k8s/resutil/pkg/config"
	"gaohoward.tools/k8s/resutil/pkg/graphics"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"k8s.io/utils/ptr"
)

const (
	MENU_ADD_NOTE    = "Add Note"
	MENU_REMOVE_NOTE = "Remove Note"
	MENU_ADD_LINK    = "Add Link"
	MENU_REMOVE_LINK = "Remove Link"

	NEW_FILE_CHOSEN        = "k8sutil.tool.reader.new-file"
	NEW_LINK_CHOSEN_BASE   = "k8sutil.tool.reader.new-link"
	NEW_NOTE_REQUEST_BASE  = "k8sutil.tool.reader.note-request"
	DELETE_BOOK_ENTRY_BASE = "k8sutil.tool.reader.delete-entry"
	DELETE_BOOK            = "k8sutil.tool.reader.delete-book"
)

type FileItem struct {
	FileUrl  string                  `yaml:"file_url"`
	RefInfos map[int]*common.RefInfo `yaml:"ref_infos"`
	BookDir  string                  `yaml:"book_dir"`
}

func newFileItem(fileUrl string, bookDir string) *FileItem {
	fileItem := &FileItem{
		FileUrl:  fileUrl,
		RefInfos: make(map[int]*common.RefInfo),
		BookDir:  bookDir,
	}
	return fileItem
}

func (f *FileItem) Save() error {
	data, err := yaml.Marshal(f)
	if err != nil {
		logger.Error("failed to marshal file item", zap.String("file", f.FileUrl), zap.Error(err))
		return err
	}
	fileHash := md5.Sum([]byte(f.FileUrl))
	filePath := filepath.Join(f.BookDir, fmt.Sprintf("%x.yaml", fileHash))
	return os.WriteFile(filePath, data, 0644)
}

func (f *FileItem) UpdateLinks(liner *common.Liner, links []*common.ExtraLink) {
	ln := liner.GetLineNumber()
	ref, ok := f.RefInfos[ln]
	if !ok {
		ref = &common.RefInfo{
			LineNumber:  ln,
			InlineNote:  "",
			RefFileUrls: []string{},
		}
		f.RefInfos[ln] = ref
	}
	ref.RefFileUrls = []string{}
	for _, link := range links {
		ref.RefFileUrls = append(ref.RefFileUrls, link.Link)
	}
	f.Save()
}

func (f *FileItem) UpdateNote(liner *common.Liner, content string) {
	ln := liner.GetLineNumber()
	ref, ok := f.RefInfos[ln]
	if !ok {
		ref = &common.RefInfo{
			LineNumber:  ln,
			InlineNote:  "",
			RefFileUrls: []string{},
		}
		f.RefInfos[ln] = ref
	}
	ref.InlineNote = content
	f.Save()
}

func (f *FileItem) AddLinkRef(liner *common.Liner, link string) {
	ln := liner.GetLineNumber()
	ref, ok := f.RefInfos[ln]
	if !ok {
		ref = &common.RefInfo{
			LineNumber:  ln,
			InlineNote:  "",
			RefFileUrls: []string{},
		}
		f.RefInfos[ln] = ref
	}
	ref.RefFileUrls = append(ref.RefFileUrls, link)

	f.Save()
}

type FilePane struct {
	item              *FileItem
	linkContextKey    string
	noteContextKey    string
	editor            *common.ReadOnlyEditor
	resize            component.Resize
	actions           []common.LineAction
	controlBar        layout.Widget
	titleLabel        material.LabelStyle
	closeBtn          widget.Clickable
	editPanel         *common.EditDialog
	noteEdit          *common.EditorDialogTarget
	linkChooserDialog *common.EditDialog
	linkChooserEdit   *common.ChoiceDialogTarget
	showAddNote       bool
	showRemoveLinks   bool
}

func (fp *FilePane) RegisterEditorListener(fn *FileNavigator) error {
	return fp.editor.RegisterEditorListener(fn)
}

func (fp *FilePane) UpdateNote(liner *common.Liner, content string) {
	fp.item.UpdateNote(liner, content)
	liner.AddNote(content, fp.editor)
}

func (fp *FilePane) AddLink(liner *common.Liner, link string) {
	fp.item.AddLinkRef(liner, link)
	liner.AddRefLink(link, fp.editor)
}

func (fn *FileNavigator) NewFilePane(item *FileItem) *FilePane {
	fp := &FilePane{
		item:           item,
		linkContextKey: NEW_LINK_CHOSEN_BASE + item.FileUrl,
		noteContextKey: NEW_NOTE_REQUEST_BASE + item.FileUrl,
	}

	th := common.GetTheme()

	common.RegisterContext(fp.linkContextKey, nil, true)
	common.RegisterContext(fp.noteContextKey, nil, true)

	fp.noteEdit = common.NewEditorDialogTarget(nil, "")
	fp.editPanel = common.NewEditDialog("note", "", "", fp.noteEdit)

	fp.linkChooserEdit = common.NewChoiceDialogTarget(nil, nil)
	fp.linkChooserDialog = common.NewEditDialog("links", "select the link", "", fp.linkChooserEdit)

	fp.titleLabel = material.Label(th, unit.Sp(14), item.FileUrl)
	fp.titleLabel.Color = common.COLOR.Blue

	fp.controlBar = func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				return fp.titleLabel.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				button := material.Button(th, &fp.closeBtn, "x")
				button.Background = th.Bg
				button.Inset = layout.Inset{Top: 2, Bottom: 2, Left: 2, Right: 2}
				button.Color = th.Fg
				button.Font.Weight = font.Bold

				return button.Layout(gtx)
			}),
		)
	}

	fp.resize.Ratio = 0.4

	fp.actions = []common.LineAction{
		NewAddNoteAction(MENU_ADD_LINK, graphics.ResIcon, fp),
		NewAddNoteAction(MENU_REMOVE_LINK, graphics.DeleteIcon, fp),
		NewAddNoteAction(MENU_ADD_NOTE, graphics.AddIcon, fp),
		NewAddNoteAction(MENU_REMOVE_NOTE, graphics.DeleteIcon, fp),
	}

	fp.editor = common.NewReadOnlyEditor("", 16, nil, fp.actions, true)
	data, err := os.ReadFile(item.FileUrl)
	if err != nil {
		errTxt := fmt.Sprintf("Failed to read file: %s\nError: %v", item.FileUrl, err)
		logger.Error("failed to read file for editor", zap.String("file", item.FileUrl), zap.Error(err))
		fp.editor.SetText(&errTxt, nil)
	} else {
		content := string(data)
		fp.editor.SetText(&content, item.RefInfos)
	}
	fp.RegisterEditorListener(fn)
	fn.allPanes[item.FileUrl] = fp
	fn.sourceEditorMap[fp.editor.GetId()] = fp
	return fp
}

func (fp *FilePane) Layout(gtx layout.Context) layout.Dimensions {

	reqData, extra := common.PollContextData(fp.noteContextKey)
	if reqData != nil {
		isDelete := ptr.To(false)
		okBool := false
		if liner, ok := reqData.(*common.Liner); ok {
			if extra != nil {
				if isDelete, okBool = extra.(*bool); okBool && *isDelete {
					//delete the note
					liner.RemoveNote()
					fp.item.UpdateNote(liner, "")
				}
			}
			if !*isDelete {
				// add the note
				targetLine := liner.GetLineNumber() + 1
				title := fmt.Sprintf("Note at line %d", targetLine)
				existing := liner.GetNote()
				fp.editPanel.SetTitle(title)
				if existing != nil {
					fp.noteEdit.SetContent(*existing)
				}
				fp.noteEdit.SetCallback(func(actionType common.ActionType, content string) {
					fp.showAddNote = false
					if actionType == common.OK {
						fp.UpdateNote(liner, content)
					}
				})
				fp.showAddNote = true
			}
		}
	}

	if fp.showAddNote {
		return fp.editPanel.Layout(gtx)
	}

	data, extra := common.PollContextData(fp.linkContextKey)
	if data != nil && extra != nil {
		if fileUrl, ok := data.(*string); ok {
			if liner, ok1 := extra.(*common.Liner); ok1 {
				if *fileUrl != "" {
					fp.AddLink(liner, *fileUrl)
				} else {
					if links := liner.GetLinks(); len(links) > 0 {
						fp.linkChooserDialog.SetTitle("Remove Links")
						fp.linkChooserDialog.SetSubtitle("select links to remove")
						choices := make([]*common.Choice, 0)
						for _, link := range links {
							choices = append(choices, &common.Choice{
								Name:  link.Link,
								Value: link,
							})
						}
						fp.linkChooserEdit.SetChoices(choices)
						fp.linkChooserEdit.SetCallback(func(actionType common.ActionType, selected []*common.Choice) {
							fp.showRemoveLinks = false
							if actionType == common.OK {
								liner.RemoveRefLinks(selected)
								fp.item.UpdateLinks(liner, liner.GetLinks())
							}
						})
						fp.showRemoveLinks = true
					}
				}
			}

		}
	}

	if fp.showRemoveLinks {
		return fp.linkChooserDialog.Layout(gtx)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fp.controlBar(gtx)
		}),
		layout.Flexed(1.0, fp.editor.Layout),
	)
}

type FileNavigator struct {
	tool            *ReaderTool
	topBar          material.LabelStyle
	rootPanesMap    map[string][]*FilePane // root -> panes
	sourceEditorMap map[string]*FilePane   //editor id -> pane
	allPanes        map[string]*FilePane   // url -> pane
}

// ExtraLinkClicked implements common.EditorEventListener.
func (fn *FileNavigator) ExtraLinkClicked(link *common.ExtraLink) {

	if sourcePane, ok := fn.sourceEditorMap[link.SourceEditor.GetId()]; ok {
		// must be current
		root := fn.tool.currentEntry
		if root == nil {
			logger.Warn("cannot find current entry")
			return
		}
		sourcePaneList, hasList := fn.rootPanesMap[root.Key()]
		if !hasList {
			logger.Warn("no pane list for root", zap.String("root", root.FilePath))
			return
		}

		index := -1
		for i, sp := range sourcePaneList {
			if sp == sourcePane {
				// found it
				index = i
				break
			}
		}

		if index == -1 {
			logger.Warn("the pane is not found")
			return
		}

		sourcePaneList = sourcePaneList[:index+1]

		//now append the pane for the link
		targetPane, targetExist := fn.allPanes[link.Link]
		if !targetExist {
			item := fn.tool.currentEntry.Book.GetFileItem(link.Link)
			targetPane = fn.NewFilePane(item)
		}
		sourcePaneList = append(sourcePaneList, targetPane)
		fn.rootPanesMap[root.Key()] = sourcePaneList
	} else {
		//error: the source pane should have already been there
		logger.Warn("no source pane found")
	}
}

type AddNoteAction struct {
	name string
	icon *widget.Icon
	Pane *FilePane
}

// FileChoosed implements common.FileHandler.
func (a *AddNoteAction) FileChoosed(fileUrl string, attachment any) error {
	if a.name != MENU_ADD_LINK {
		return fmt.Errorf("wrong action type %v", a.name)
	}
	common.SetContextData(a.Pane.linkContextKey, &fileUrl, attachment)
	return nil
}

// GetFilter implements common.FileHandler.
func (a *AddNoteAction) GetFilter() []string {
	return []string{}
}

// Execute implements common.MenuAction.
func (a *AddNoteAction) Execute(gtx layout.Context, se *common.ReadOnlyEditor, target *common.Liner) error {
	switch a.name {
	case MENU_ADD_NOTE:
		a.DoAddNote(gtx, se, target)
	case MENU_REMOVE_NOTE:
		a.DoRemoveNote(gtx, se, target)
	case MENU_ADD_LINK:
		a.DoAddLink(gtx, se, target)
	case MENU_REMOVE_LINK:
		a.DoRemoveLink(gtx, se, target)
	default:
		logger.Warn("Unknown action executed", zap.String("action", a.name))
	}
	return nil
}

func (a AddNoteAction) DoRemoveLink(gtx layout.Context, se *common.ReadOnlyEditor, target *common.Liner) {
	common.SetContextData(a.Pane.linkContextKey, ptr.To(""), target)
}

func (a *AddNoteAction) DoAddLink(gtx layout.Context, se *common.ReadOnlyEditor, target *common.Liner) {
	go common.AsyncChooseFile(a, target)
}

func (a *AddNoteAction) DoAddNote(gtx layout.Context, se *common.ReadOnlyEditor, target *common.Liner) {
	common.SetContextData(a.Pane.noteContextKey, target, nil)
}

func (a *AddNoteAction) DoRemoveNote(gtx layout.Context, se *common.ReadOnlyEditor, target *common.Liner) {
	common.SetContextData(a.Pane.noteContextKey, target, ptr.To(true))
}

// GetName implements common.MenuAction.
func (a *AddNoteAction) GetName() string {
	return a.name
}

func (a *AddNoteAction) GetIcon() *widget.Icon {
	return a.icon
}

func NewAddNoteAction(name string, icon *widget.Icon, pane *FilePane) common.LineAction {
	a := &AddNoteAction{
		name: name,
		icon: icon,
		Pane: pane,
	}
	return a
}

func NewFileNavigator(t *ReaderTool, book *Book) *FileNavigator {
	fn := &FileNavigator{tool: t}
	fn.rootPanesMap = make(map[string][]*FilePane)
	fn.sourceEditorMap = make(map[string]*FilePane)
	fn.allPanes = make(map[string]*FilePane)
	fn.topBar = material.Label(common.GetTheme(), unit.Sp(16), "File Navigator")
	fn.topBar.Font.Weight = font.Bold
	return fn
}

func (fn *FileNavigator) layoutFilePanes(gtx layout.Context, panes []*FilePane, pos int) layout.Dimensions {

	if len(panes) == 1 {
		return panes[pos].Layout(gtx)
	}
	if pos == len(panes)-1 {
		return panes[pos].Layout(gtx)
	}
	return panes[pos].resize.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return panes[pos].Layout(gtx)
		},
		func(gtx layout.Context) layout.Dimensions {
			return fn.layoutFilePanes(gtx, panes, pos+1)
		},
		common.VerticalSplitHandler)
}

// make a vertical flex with a top bar rigid
// and a flexd holding a horizontal resizable panes
func (fn *FileNavigator) Layout(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// top bar
			if fn.tool.currentEntry != nil {
				fn.topBar.Text = fmt.Sprintf("%s :: %s", fn.tool.currentEntry.Book.Name, filepath.Base(fn.tool.currentEntry.FilePath))
			} else {
				fn.topBar.Text = "No file opened"
			}
			return fn.topBar.Layout(gtx)
		}),
		layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
			if fn.tool.currentEntry == nil {
				return layout.Dimensions{}
			}

			filePanes, ok := fn.rootPanesMap[fn.tool.currentEntry.Key()]

			if !ok {
				// create the first pane
				filePanes = make([]*FilePane, 0)
				fileItem, ok := fn.tool.currentEntry.Book.fileSet[fn.tool.currentEntry.FilePath]
				if !ok {
					logger.Error("file item not found for current entry", zap.String("file", fn.tool.currentEntry.FilePath))
					return layout.Dimensions{}
				}
				firstPane := fn.NewFilePane(fileItem)
				fn.sourceEditorMap[firstPane.editor.GetId()] = firstPane
				fn.allPanes[firstPane.item.FileUrl] = firstPane
				filePanes = append(filePanes, firstPane)
				fn.rootPanesMap[fn.tool.currentEntry.Key()] = filePanes
			}
			return fn.layoutFilePanes(gtx, filePanes, 0)
		}),
	)
}

type RootEntry struct {
	Book      *Book
	FilePath  string
	Clickable widget.Clickable

	deleteClickable widget.Clickable
	menuState       component.MenuState
	menuContextArea component.ContextArea
}

func (r *RootEntry) Layout(gtx layout.Context) layout.Dimensions {

	if r.deleteClickable.Clicked(gtx) {
		r.Book.DeleteEntry(r)
	}

	label := material.Label(common.GetTheme(), unit.Sp(16), filepath.Base(r.FilePath))
	if r.Clickable.Clicked(gtx) {
		if r.Book.tool.currentEntry != r {
			r.Book.tool.currentEntry = r
		}
	}
	if r == r.Book.tool.currentEntry {
		label.Color = color.NRGBA{R: 0, G: 0, B: 255, A: 255}
		label.Font.Weight = font.Bold
	} else {
		label.Color = color.NRGBA{R: 0, G: 0, B: 0, A: 255}
		label.Font.Weight = font.Normal
	}
	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return material.Clickable(gtx, &r.Clickable, func(gtx layout.Context) layout.Dimensions {
				return label.Layout(gtx)
			})
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return r.menuContextArea.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Point{}
				return component.Menu(common.GetTheme(), &r.menuState).Layout(gtx)
			})
		}),
	)

}

func (r *RootEntry) Key() string {
	return r.Book.Name + "//" + r.FilePath
}

func newRootEntry(owner *Book, path string) *RootEntry {
	r := &RootEntry{
		Book:     owner,
		FilePath: path,
	}
	r.menuState.Options = []func(gtx layout.Context) layout.Dimensions{
		func(gtx layout.Context) layout.Dimensions {
			return common.ItemFunc(gtx, &r.deleteClickable, "Delete", graphics.DeleteIcon)
		},
	}
	return r
}

type Book struct {
	Name                  string
	repo                  *BookRepo
	bookClickable         widget.Clickable
	newRootFileClickable  widget.Clickable
	deleteClickable       widget.Clickable
	deleteEntryContextKey string
	bookDir               string
	tool                  *ReaderTool
	fileList              []*RootEntry
	fileSet               map[string]*FileItem
	fileWidgetList        widget.List
	discloserState        component.DiscloserState
	menuState             component.MenuState
	menuContextArea       component.ContextArea
}

func (b *Book) DeleteEntry(entry *RootEntry) {
	common.SetContextData(b.deleteEntryContextKey, entry, nil)
}

func (b *Book) Layout(gtx layout.Context, isCurrent bool) layout.Dimensions {
	if b.deleteClickable.Clicked(gtx) {
		b.repo.DeleteBook(b)
	}
	data, _ := common.PollContextData(b.deleteEntryContextKey)
	if data != nil {
		if toDel, ok := data.(*RootEntry); ok {
			logger.Info("removing entry", zap.String("name", toDel.FilePath))
			b.fileList = slices.DeleteFunc(b.fileList, func(elem *RootEntry) bool {
				return toDel == elem
			})
			b.save()
		}
	}
	th := common.GetTheme()
	return component.SimpleDiscloser(th, &b.discloserState).Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Stack{}.Layout(gtx,
						layout.Stacked(func(gtx layout.Context) layout.Dimensions {
							return ClickableLabel(gtx, &b.bookClickable, b.Name, isCurrent)
						}),
						layout.Expanded(func(gtx layout.Context) layout.Dimensions {
							return b.menuContextArea.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								gtx.Constraints.Min = image.Point{}
								return component.Menu(th, &b.menuState).Layout(gtx)
							})
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if b.newRootFileClickable.Clicked(gtx) {
						b.NewRootFile()
					}
					gtx.Constraints.Max.X = 16
					gtx.Constraints.Max.Y = 16
					return layout.Inset{Top: 2, Bottom: 0, Left: 0, Right: 0}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Clickable(gtx, &b.newRootFileClickable, func(gtx layout.Context) layout.Dimensions {
							return graphics.OpenNewIcon.Layout(gtx, common.COLOR.DarkGreen)
						})
					})
				}),
			)
		},
		func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &b.fileWidgetList).Layout(gtx, len(b.fileList),
				func(gtx layout.Context, index int) layout.Dimensions {
					return b.fileList[index].Layout(gtx)
				})
		},
	)
}

func (b *Book) save() {
	if _, err := os.Stat(b.bookDir); os.IsNotExist(err) {
		err := os.MkdirAll(b.bookDir, 0755)
		if err != nil {
			logger.Warn("cannot create bookdir", zap.String("path", b.bookDir), zap.Error(err))
			return
		}
	}
	masterPath := filepath.Join(b.bookDir, "master.idx")
	var b64Paths []string
	for _, path := range b.fileList {
		encodedPath := base64.StdEncoding.EncodeToString([]byte(path.FilePath))
		b64Paths = append(b64Paths, encodedPath)
	}
	masterData := strings.Join(b64Paths, " ")
	err := os.WriteFile(masterPath, []byte(masterData), 0644)
	if err != nil {
		logger.Error("failed to write master.idx", zap.Error(err))
		return
	}
	for _, fileItem := range b.fileSet {
		if err := fileItem.Save(); err != nil {
			logger.Error("failed to write file item", zap.String("file", fileItem.FileUrl), zap.Error(err))
		}
	}
}

// the book dir contains saved files
// all files except the file called master.idx are considered saved files
// their names are sha/md5 hashes of the full file path
// each such file is a serialized FileItem struct in yaml format
// the master.idx has the list of root files by their full paths
// those are the user initiated files as the beginning of reading.
func (b *Book) load() {
	files, err := os.ReadDir(b.bookDir)
	if err != nil {
		logger.Error("failed to read reader tool dir", zap.Error(err))
		return
	}

	for _, file := range files {
		if !file.IsDir() {
			if file.Name() == "master.idx" {
				// the file has a list of root file paths encoded in base64
				masterPath := filepath.Join(b.bookDir, file.Name())
				data, err := os.ReadFile(masterPath)
				if err != nil {
					logger.Error("failed to read master.idx", zap.Error(err))
					return
				}
				b64Paths := strings.SplitSeq(strings.TrimSpace(string(data)), " ")
				for b64Path := range b64Paths {
					if strings.TrimSpace(b64Path) == "" {
						continue
					}
					decodedPath, err := base64.StdEncoding.DecodeString(b64Path)
					if err != nil {
						logger.Error("failed to decode base64 file path", zap.String("b64", b64Path), zap.Error(err))
						continue
					}
					fullPath := string(decodedPath)
					b.fileList = append(b.fileList, newRootEntry(b, fullPath))
				}
			} else {
				filePath := filepath.Join(b.bookDir, file.Name())
				data, err := os.ReadFile(filePath)
				if err != nil {
					logger.Error("failed to read file", zap.String("file", file.Name()), zap.Error(err))
					continue
				}
				var fileItem FileItem
				err = yaml.Unmarshal(data, &fileItem)
				if err != nil {
					logger.Error("failed to unmarshal file item", zap.String("file", file.Name()), zap.Error(err))
					continue
				}
				if b.fileSet == nil {
					b.fileSet = make(map[string]*FileItem)
				}
				b.fileSet[fileItem.FileUrl] = &fileItem
			}
		}
	}
}

func NewBook(name string, bookDir string, tool *ReaderTool) *Book {
	b := &Book{
		tool:     tool,
		Name:     name,
		bookDir:  bookDir,
		fileList: make([]*RootEntry, 0),
		fileSet:  make(map[string]*FileItem),
	}
	b.fileWidgetList.Axis = layout.Vertical
	b.menuState.Options = []func(gtx layout.Context) layout.Dimensions{
		func(gtx layout.Context) layout.Dimensions {
			return common.ItemFunc(gtx, &b.deleteClickable, "Delete", graphics.DeleteIcon)
		},
	}
	b.deleteEntryContextKey = DELETE_BOOK_ENTRY_BASE + "//" + name
	common.RegisterContext(b.deleteEntryContextKey, nil, true)
	return b
}

type BookRepo struct {
	tool           *ReaderTool
	repoDir        string
	books          []*Book
	currentBook    *Book
	bookWidgetList widget.List

	nameChooser *common.EditDialog
	nameEdit    *common.OptionDialogTarget
	showNewBook bool
}

func (br *BookRepo) DeleteBook(b *Book) {
	common.SetContextData(DELETE_BOOK, b, nil)
}

func (b *BookRepo) Layout(gtx layout.Context) layout.Dimensions {
	data, _ := common.PollContextData(DELETE_BOOK)
	if data != nil {
		if book, ok := data.(*Book); ok {
			if len(book.fileList) == 0 {
				b.books = slices.DeleteFunc(b.books, func(b *Book) bool {
					return book == b
				})
				os.RemoveAll(book.bookDir)
			} else {
				logger.Info("book not empty, not to delete", zap.String("book name", book.Name))
			}
		}
	}
	if b.showNewBook {
		return b.nameChooser.Layout(gtx)
	}
	return b.bookWidgetList.Layout(gtx, len(b.books), func(gtx layout.Context, index int) layout.Dimensions {
		book := b.books[index]
		if book.bookClickable.Clicked(gtx) {
			if b.currentBook != book {
				b.currentBook = book
			}
		}
		return book.Layout(gtx, book == b.currentBook)
	})
}

func (b *BookRepo) CreateNewBook() {
	keys := []string{"Name"}
	defValues := []string{b.GenerateNewBookName()}
	desc := []string{"name of the book"}
	b.nameChooser.SetTitle("New Book")
	b.nameChooser.SetSubtitle("book name must be unique")
	b.nameEdit.SetOptions(keys, defValues, desc)
	b.nameEdit.SetCallback(func(actionType common.ActionType, options map[string]string) {
		b.showNewBook = false
		if actionType == common.OK {
			name := options["Name"]
			b.newBook(name)
		}
	})
	b.showNewBook = true
}

func (b *BookRepo) GenerateNewBookName() string {
	count := 1
	var bookName string
	for {
		bookName = fmt.Sprintf("book%d", count)
		exists := false
		for _, b := range b.books {
			if b.Name == bookName {
				exists = true
				break
			}
		}
		if !exists {
			break
		}
		count++
	}
	return bookName
}

func (b *BookRepo) newBook(name string) {
	bookPath := filepath.Join(b.repoDir, name)
	book := NewBook(name, bookPath, b.tool)
	b.books = append(b.books, book)
}

func (b *BookRepo) loadBooks() error {
	entries, err := os.ReadDir(b.repoDir)
	if err != nil {
		return err
	}
	for _, ent := range entries {
		if ent.IsDir() {
			fullPath := filepath.Join(b.repoDir, ent.Name())
			book := NewBook(ent.Name(), fullPath, b.tool)
			book.load()
			b.books = append(b.books, book)
		}
	}
	return nil
}

func NewBookRepo(baseDir string, tool *ReaderTool) *BookRepo {
	r := &BookRepo{
		tool:    tool,
		repoDir: baseDir,
		books:   make([]*Book, 0),
	}
	r.bookWidgetList.Axis = layout.Vertical

	r.nameEdit = common.NewOptionDialogTarget(nil, nil, nil)
	r.nameChooser = common.NewEditDialog("", "", "", r.nameEdit)

	common.RegisterContext(DELETE_BOOK, nil, true)

	return r
}

type ReaderTool struct {
	BaseDir   string
	widget    layout.Widget
	clickable widget.Clickable // for the tools tab to activate this tool
	resize    component.Resize

	newBookTooltip    component.Tooltip
	newBookClickable  widget.Clickable
	newBookBtnTipArea component.TipArea

	bookRepo *BookRepo

	fileNavigator *FileNavigator

	currentEntry *RootEntry
}

func (b *Book) GetFileItem(fileUrl string) *FileItem {
	item, ok := b.fileSet[fileUrl]
	if !ok {
		item = newFileItem(fileUrl, b.bookDir)
		b.fileSet[fileUrl] = item
	}
	return item
}

func (c *ReaderTool) loadBooks() {
	//under baseDir, each folder is a book
	c.bookRepo = NewBookRepo(c.BaseDir, c)
	c.bookRepo.loadBooks()
}

func (c *ReaderTool) updateFiles() {
	data, extra := common.PollContextData(NEW_FILE_CHOSEN)
	if data != nil {
		if extra == nil {
			logger.Info("can't get the book")
			return
		}
		if book, ok := extra.(*Book); ok {

			if fileUrl, ok := data.(*string); ok {
				if *fileUrl != "" {
					book.GetFileItem(*fileUrl)
					rootEntry := newRootEntry(book, *fileUrl)
					c.currentEntry = rootEntry
					book.fileList = append(book.fileList, rootEntry)
					c.Save()
				}
			}
		}

	}
}

// FileChoosed implements common.FileHandler.
func (b *Book) FileChoosed(fileUrl string, _ any) error {
	common.SetContextData(NEW_FILE_CHOSEN, &fileUrl, b)
	return nil
}

// GetFilter implements common.FileHandler.
func (b *Book) GetFilter() []string {
	return []string{}
}

func (b *Book) NewRootFile() {
	go common.AsyncChooseFile(b, nil)
}

func (c *ReaderTool) Save() {
	for _, book := range c.bookRepo.books {
		book.save()
	}
}

func (c *ReaderTool) GetBarButtons() []layout.FlexChild {
	children := make([]layout.FlexChild, 0)
	button := component.TipIconButtonStyle{
		Tooltip:         c.newBookTooltip,
		IconButtonStyle: material.IconButton(common.GetTheme(), &c.newBookClickable, graphics.AddIcon, "Open File"),
		State:           &c.newBookBtnTipArea,
	}

	button.Size = 20
	button.IconButtonStyle.Inset = layout.Inset{Top: 1, Bottom: 1, Left: 1, Right: 1}
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(10)}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				if c.newBookClickable.Clicked(gtx) {
					c.bookRepo.CreateNewBook()
				}
				return button.Layout(gtx)
			},
		)
	}))

	return children
}

// GetClickable implements Tool.
func (c *ReaderTool) GetClickable() *widget.Clickable {
	return &c.clickable
}

// GetName implements Tool.
func (c *ReaderTool) GetName() string {
	return "reader"
}

// GetTabButtons implements Tool.
func (c *ReaderTool) GetTabButtons() []layout.FlexChild {
	return nil
}

// GetWidget implements Tool.
func (c *ReaderTool) GetWidget() layout.Widget {
	return c.widget
}

func (c *ReaderTool) Init() error {
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config %v", err)
	}
	// Use cfg to load files as needed
	readerDir, err := cfg.GetToolDir("reader")
	if err != nil {
		return fmt.Errorf("failed to get reader tool dir: %v", err)
	}
	c.BaseDir = readerDir

	common.RegisterContext(NEW_FILE_CHOSEN, nil, true)

	return nil
}

func NewReaderTool() (Tool, error) {
	c := &ReaderTool{}

	if err := c.Init(); err != nil {
		return nil, err
	}

	c.resize.Ratio = 0.25
	c.newBookTooltip = component.DesktopTooltip(common.GetTheme(), "new book")

	c.fileNavigator = NewFileNavigator(c, nil)

	c.loadBooks()

	bttns := c.GetBarButtons()

	c.widget = func(gtx layout.Context) layout.Dimensions {
		c.updateFiles()
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			// the vertial action bar
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(0), Bottom: unit.Dp(0), Left: unit.Dp(0), Right: unit.Dp(0)}.Layout(gtx,
					func(gtx layout.Context) layout.Dimensions {
						return layout.Stack{Alignment: layout.NW}.Layout(gtx,
							// the bar
							layout.Stacked(func(gtx layout.Context) layout.Dimensions {
								barHeight := gtx.Constraints.Max.Y
								defWidth := gtx.Dp(unit.Dp(24))
								maxWidth := gtx.Constraints.Max.X
								barWidth := min(defWidth, maxWidth)
								barRect := image.Rect(0, 0, barWidth, barHeight)
								barColor := color.NRGBA{R: 224, G: 224, B: 224, A: 255}
								paint.FillShape(gtx.Ops, barColor, clip.Rect(barRect).Op())
								return layout.Dimensions{
									Size: image.Point{X: barWidth, Y: barHeight},
								}
							}),
							// the buttons on the bar
							layout.Stacked(func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									bttns...,
								)
							}),
						)
					},
				)
			}),
			// the file content area
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				// horizontal resize with left as the file list and right as the file navigator
				return c.resize.Layout(gtx,
					func(gtx layout.Context) layout.Dimensions {
						// left: book list
						return c.bookRepo.Layout(gtx)
					},
					func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return c.fileNavigator.Layout(gtx)
						})
					},
					common.VerticalSplitHandler,
				)
			}),
		)
	}
	return c, nil
}
