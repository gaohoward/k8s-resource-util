package panels

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
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
)

const (
	MENU_ADD_NOTE = "Add Note"
	MENU_ADD_LINK = "Add Link"
)

type FileItem struct {
	FileUrl   string                  `yaml:"file_url"`
	RefInfos  map[int]*common.RefInfo `yaml:"ref_infos"`
	ReaderDir string                  `yaml:"reader_dir"`
}

func newFileItem(fileUrl string, readerDir string) *FileItem {
	fileItem := &FileItem{
		FileUrl:  fileUrl,
		RefInfos: make(map[int]*common.RefInfo),
	}
	//get readerDir from fileUrl
	fileItem.ReaderDir = readerDir
	return fileItem
}

func (f *FileItem) Save() error {
	data, err := yaml.Marshal(f)
	if err != nil {
		logger.Error("failed to marshal file item", zap.String("file", f.FileUrl), zap.Error(err))
		return err
	}
	fileHash := md5.Sum([]byte(f.FileUrl))
	filePath := filepath.Join(f.ReaderDir, fmt.Sprintf("%x.yaml", fileHash))
	return os.WriteFile(filePath, data, 0644)
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
	item       *FileItem
	editor     *common.ReadOnlyEditor
	resize     component.Resize
	actions    []common.MenuAction
	controlBar layout.Widget
	titleLabel material.LabelStyle
	closeBtn   widget.Clickable
}

func (fp *FilePane) RegisterEditorListener(fn *FileNavigator) error {
	return fp.editor.RegisterEditorListener(fn)
}

func (fp *FilePane) AddLink(liner *common.Liner, link string) {
	fp.item.AddLinkRef(liner, link)
	liner.AddRefLink(link, fp.editor)
}

func (fn *FileNavigator) NewFilePane(th *material.Theme, item *FileItem) *FilePane {
	fp := &FilePane{
		item: item,
	}

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

	fp.actions = []common.MenuAction{
		NewAddNoteAction(MENU_ADD_LINK, th, graphics.ResIcon, fp),
		NewAddNoteAction(MENU_ADD_NOTE, th, graphics.AddIcon, fp),
	}

	fp.editor = common.NewReadOnlyEditor(th, "", 16, fp.actions, true)
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
		sourcePaneList, hasList := fn.rootPanesMap[root.FilePath]
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
			item := fn.tool.GetFileItem(link.Link)
			targetPane = fn.NewFilePane(link.SourceEditor.Theme(), item)
		}
		sourcePaneList = append(sourcePaneList, targetPane)
		fn.rootPanesMap[root.FilePath] = sourcePaneList
	} else {
		//error: the source pane should have already been there
		logger.Warn("no source pane found")
	}
}

type AddNoteAction struct {
	name      string
	clickable widget.Clickable
	menuFunc  func(gtx layout.Context) layout.Dimensions
	Pane      *FilePane
}

// Execute implements common.MenuAction.
func (a *AddNoteAction) Execute(gtx layout.Context, se *common.ReadOnlyEditor) error {
	switch a.name {
	case MENU_ADD_NOTE:
		a.DoAddNote(gtx, se)
	case MENU_ADD_LINK:
		a.DoAddLink(gtx, se)
	default:
		logger.Warn("Unknown action executed", zap.String("action", a.name))
	}
	return nil
}

func (a *AddNoteAction) DoAddLink(gtx layout.Context, se *common.ReadOnlyEditor) {
	lines := se.GetSelectedLines()
	if len(lines) == 0 {
		return
	}
	// for simplicity, we just take the first selected line
	// pop up a the file chooser dialog to select a file
	explorer := common.GetExplorer()
	reader, err := explorer.ChooseFile(".sh", ".go", ".py")

	if err != nil {
		return
	}
	if reader == nil {
		return
	}
	defer reader.Close()

	if file, ok := reader.(*os.File); ok {
		a.Pane.AddLink(lines[0], file.Name())
	} else {
		logger.Info("cannot get file name")
		return
	}
}

func (a *AddNoteAction) DoAddNote(gtx layout.Context, se *common.ReadOnlyEditor) {
}

// GetClickable implements common.MenuAction.
func (a *AddNoteAction) GetClickable() *widget.Clickable {
	return &a.clickable
}

// GetMenuOption implements common.MenuAction.
func (a *AddNoteAction) GetMenuOption(th *material.Theme) func(gtx layout.Context) layout.Dimensions {
	return a.menuFunc
}

// GetName implements common.MenuAction.
func (a *AddNoteAction) GetName() string {
	return a.name
}

func NewAddNoteAction(name string, th *material.Theme, icon *widget.Icon, pane *FilePane) common.MenuAction {
	a := &AddNoteAction{name: name, Pane: pane}
	a.menuFunc = func(gtx layout.Context) layout.Dimensions {
		return common.ItemFunc(th, gtx, &a.clickable, a.name, icon)
	}
	return a
}

func NewFileNavigator(th *material.Theme, t *ReaderTool) *FileNavigator {
	fn := &FileNavigator{tool: t}
	fn.rootPanesMap = make(map[string][]*FilePane)
	fn.sourceEditorMap = make(map[string]*FilePane)
	fn.allPanes = make(map[string]*FilePane)
	fn.topBar = material.Label(th, unit.Sp(16), "File Navigator")
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
func (fn *FileNavigator) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// top bar
			if fn.tool.currentEntry != nil {
				fn.topBar.Text = fmt.Sprintf("File: %s", fn.tool.currentEntry.FilePath)
			} else {
				fn.topBar.Text = "No file opened"
			}
			return fn.topBar.Layout(gtx)
		}),
		layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
			// horizontal resizable panes
			if fn.tool.currentEntry == nil {
				return layout.Dimensions{}
			}
			filePanes, ok := fn.rootPanesMap[fn.tool.currentEntry.FilePath]

			if !ok {
				// create the first pane
				filePanes = make([]*FilePane, 0)
				fileItem, ok := fn.tool.fileSet[fn.tool.currentEntry.FilePath]
				if !ok {
					logger.Error("file item not found for current entry", zap.String("file", fn.tool.currentEntry.FilePath))
					return layout.Dimensions{}
				}
				firstPane := fn.NewFilePane(th, fileItem)
				fn.sourceEditorMap[firstPane.editor.GetId()] = firstPane
				fn.allPanes[firstPane.item.FileUrl] = firstPane
				filePanes = append(filePanes, firstPane)
				fn.rootPanesMap[fn.tool.currentEntry.FilePath] = filePanes
			}
			return fn.layoutFilePanes(gtx, filePanes, 0)
		}),
	)
}

type RootEntry struct {
	FilePath  string
	Clickable widget.Clickable
}

func newRootEntry(path string) *RootEntry {
	return &RootEntry{FilePath: path}
}

type ReaderTool struct {
	BaseDir   string
	widget    layout.Widget
	clickable widget.Clickable
	resize    component.Resize

	newFileTooltip    component.Tooltip
	newFileClickable  widget.Clickable
	newFileBtnTipArea component.TipArea

	fileNavigator *FileNavigator

	currentEntry   *RootEntry
	fileList       []*RootEntry
	fileSet        map[string]*FileItem
	fileWidgetList widget.List
}

func (c *ReaderTool) GetFileItem(fileUrl string) *FileItem {
	item, ok := c.fileSet[fileUrl]
	if !ok {
		item = newFileItem(fileUrl, c.BaseDir)
		c.fileSet[fileUrl] = item
	}
	return item
}

// the /reader dir contains saved files
// all files except the file called master.idx are considered saved files
// their names are sha/md5 hashes of the full file path
// each such file is a serialized FileItem struct in yaml format
// the master.idx has the list of root files by their full paths
// those are the user initiated files as the beginning of reading.
func (c *ReaderTool) loadFiles() {
	files, err := os.ReadDir(c.BaseDir)
	if err != nil {
		logger.Error("failed to read reader tool dir", zap.Error(err))
		return
	}
	for _, file := range files {
		if !file.IsDir() {
			if file.Name() == "master.idx" {
				// the file has a list of root file paths encoded in base64
				masterPath := filepath.Join(c.BaseDir, file.Name())
				data, err := os.ReadFile(masterPath)
				if err != nil {
					logger.Error("failed to read master.idx", zap.Error(err))
					return
				}
				b64Paths := strings.SplitSeq(string(data), " ")
				for b64Path := range b64Paths {
					decodedPath, err := base64.StdEncoding.DecodeString(b64Path)
					if err != nil {
						logger.Error("failed to decode base64 file path", zap.String("b64", b64Path), zap.Error(err))
						continue
					}
					fullPath := string(decodedPath)
					c.fileList = append(c.fileList, newRootEntry(fullPath))
				}
			} else {
				filePath := filepath.Join(c.BaseDir, file.Name())
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
				if c.fileSet == nil {
					c.fileSet = make(map[string]*FileItem)
				}
				c.fileSet[fileItem.FileUrl] = &fileItem
			}
		}
	}
}

func (c *ReaderTool) NewRootFile() {
	explorer := common.GetExplorer()
	reader, err := explorer.ChooseFile(".sh", ".go", ".py")

	if err != nil {
		return
	}
	if reader == nil {
		return
	}
	defer reader.Close()

	if file, ok := reader.(*os.File); ok {
		if _, ok := c.fileSet[file.Name()]; ok {
			logger.Info("file already opened", zap.String("file", file.Name()))
			return
		}
		fileItem := c.GetFileItem(file.Name())
		rootEntry := newRootEntry(file.Name())
		c.currentEntry = rootEntry
		c.fileList = append(c.fileList, rootEntry)
		c.fileSet[file.Name()] = fileItem
		c.Save()
	} else {
		logger.Info("cannot get file name")
		return
	}
}

func (c *ReaderTool) Save() {
	cfg, err := config.GetConfig()
	if err != nil {
		logger.Error("failed to load config", zap.Error(err))
		return
	}
	readerDir, err := cfg.GetToolDir("reader")
	if err != nil {
		logger.Error("failed to get reader tool dir", zap.Error(err))
		return
	}
	masterPath := filepath.Join(readerDir, "master.idx")
	var b64Paths []string
	for _, path := range c.fileList {
		encodedPath := base64.StdEncoding.EncodeToString([]byte(path.FilePath))
		b64Paths = append(b64Paths, encodedPath)
	}
	masterData := strings.Join(b64Paths, " ")
	err = os.WriteFile(masterPath, []byte(masterData), 0644)
	if err != nil {
		logger.Error("failed to write master.idx", zap.Error(err))
		return
	}
	for _, fileItem := range c.fileSet {
		if err := fileItem.Save(); err != nil {
			logger.Error("failed to write file item", zap.String("file", fileItem.FileUrl), zap.Error(err))
		}
	}
}

func (c *ReaderTool) GetBarButtons(th *material.Theme) []layout.FlexChild {
	children := make([]layout.FlexChild, 0)
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(10)}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				if c.newFileClickable.Clicked(gtx) {
					c.NewRootFile()
				}
				button := component.TipIconButtonStyle{
					Tooltip:         c.newFileTooltip,
					IconButtonStyle: material.IconButton(th, &c.newFileClickable, graphics.AddIcon, "Open File"),
					State:           &c.newFileBtnTipArea,
				}

				button.Size = 20
				button.IconButtonStyle.Inset = layout.Inset{Top: 1, Bottom: 1, Left: 1, Right: 1}
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
func (c *ReaderTool) GetTabButtons(th *material.Theme) []layout.FlexChild {
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
		return fmt.Errorf("failed to get reader tool dir", err)
	}
	c.BaseDir = readerDir
	return nil
}

func NewReaderTool(th *material.Theme) (Tool, error) {
	c := &ReaderTool{}

	if err := c.Init(); err != nil {
		return nil, err
	}

	c.resize.Ratio = 0.25
	c.newFileTooltip = component.DesktopTooltip(th, "Open File")
	c.fileWidgetList.Axis = layout.Vertical

	c.fileSet = make(map[string]*FileItem)
	c.fileList = make([]*RootEntry, 0)
	c.fileNavigator = NewFileNavigator(th, c)

	c.loadFiles()

	c.widget = func(gtx layout.Context) layout.Dimensions {
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
									c.GetBarButtons(th)...,
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
						// left: file list
						return c.fileWidgetList.Layout(gtx, len(c.fileList), func(gtx layout.Context, index int) layout.Dimensions {
							entry := c.fileList[index]
							label := material.Label(th, unit.Sp(16), filepath.Base(entry.FilePath))
							if entry.Clickable.Clicked(gtx) {
								if c.currentEntry != entry {
									c.currentEntry = entry
								}
							}
							if entry == c.currentEntry {
								label.Color = color.NRGBA{R: 0, G: 0, B: 255, A: 255}
								label.Font.Weight = font.Bold
							} else {
								label.Color = color.NRGBA{R: 0, G: 0, B: 0, A: 255}
								label.Font.Weight = font.Normal
							}
							return material.Clickable(gtx, &entry.Clickable, func(gtx layout.Context) layout.Dimensions {
								return label.Layout(gtx)
							})
						})
					},
					func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return c.fileNavigator.Layout(gtx, th)
						})
					},
					common.VerticalSplitHandler,
				)
			}),
		)
	}
	return c, nil
}
