package panels

import (
	"encoding/base64"
	"image"
	"image/color"
	"time"

	"gaohoward.tools/k8s/resutil/pkg/common"
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
)

type Tool interface {
	GetClickable() *widget.Clickable
	GetName() string
	GetTabButtons(th *material.Theme) []layout.FlexChild
	GetWidget() layout.Widget
}

type ToolsTab struct {
	toolsTab widget.Clickable

	clearBtn        widget.Clickable
	clearBtnTooltip component.Tooltip
	clearBtnTipArea component.TipArea

	rigidButtons []layout.FlexChild

	//ConvertTool
	tools []Tool

	currentTool Tool
	toolsList   layout.List

	widget layout.Widget
}

// GetClickable implements PanelTab.
func (p *ToolsTab) GetClickable() *widget.Clickable {
	return &p.toolsTab
}

// GetTabButtons implements PanelTab.
func (p *ToolsTab) GetTabButtons(th *material.Theme) []layout.FlexChild {
	return p.rigidButtons
}

// GetTitle implements PanelTab.
func (p *ToolsTab) GetTitle() string {
	return "tools"
}

// GetWidget implements PanelTab.
func (p *ToolsTab) GetWidget() layout.Widget {
	return p.widget
}

type SourceType string

var (
	TextType SourceType = "text"
	BinType  SourceType = "binary"
)

type Source struct {
	sourceType SourceType
	err        error
	content    []byte
	writable   bool
}

func (s *Source) getString() string {
	if s.err != nil {
		return s.err.Error()
	}
	return string(s.content)
}

// the converter is like an encoder/decoder
// it takes in a source and output another source
// examples like jwt generate/decode
// sha256, md5, cert parsing, base64 encode/decode
// etc
type Converter interface {
	Convert(source *Source) *Source
	GetName() string
	GetType() ConvertKind
	RequiredConfigs() (string, string, []string, []string)
}

type Conversion struct {
	CommonNodeBase
	origin        *Conversion
	converter     Converter
	convertResult *Source
}

// readonly is used to control the editability of the source
// for root it is always writable
func (cv *Conversion) IsReadOnly() bool {
	return cv.origin != nil && cv.origin.IsReadOnly()
}

func (cv *Conversion) SetValue(value []byte) {
	if cv.IsReadOnly() {
		logger.Warn("Cannot set value on readonly conversion")
	}
	cv.convertResult.content = value
}

func (cv *Conversion) GetValue() *Source {
	if cv.convertResult == nil {
		cv.doConversion()
	}
	return cv.convertResult
}

func (cv *Conversion) GetSource() *Source {
	if cv.origin != nil {
		return cv.origin.GetValue()
	}
	return nil
}

func (cv *Conversion) UpdateContent(content string) {
	if !cv.IsReadOnly() {
		if cv.origin != nil {
			cv.origin.SetValue([]byte(content))
			cv.doConversion()
		} else {
			// root
			cv.SetValue([]byte(content))
		}
	}
}

func (cv *Conversion) doConversion() {
	if cv.origin != nil {
		result := cv.converter.Convert(cv.origin.GetValue())
		cv.convertResult = result
	}
}

func (cv *Conversion) GetValueAsString() string {
	cv.doConversion()
	if cv.convertResult.err != nil {
		return cv.convertResult.err.Error()
	}
	return cv.convertResult.getString()
}

func (cv *Conversion) GetSourceContent() string {
	if cv.origin != nil {
		return cv.origin.GetValueAsString()
	}
	return cv.convertResult.getString()
}

func (cv *Conversion) GetConvertKind() ConvertKind {
	return cv.converter.GetType()
}

func (cv *Conversion) GetSourceName() string {
	if cv.origin != nil {
		return cv.origin.GetName()
	}
	// root
	return cv.converter.GetName()
}

func (cv *Conversion) GetName() string {
	return cv.converter.GetName()
}

type JwtConverter struct {
}

// RequiredConfigs implements Converter.
func (j *JwtConverter) RequiredConfigs() (string, string, []string, []string) {
	return "Jwt Config", "", []string{"algorithm"}, nil
}

// GetType implements Converter.
func (j *JwtConverter) GetType() ConvertKind {
	return jwtKind
}

// Convert implements Converter.
func (j JwtConverter) Convert(source *Source) *Source {
	return &Source{
		content:    []byte("not implemented"),
		sourceType: TextType,
	}
}

// GetName implements Converter.
func (j JwtConverter) GetName() string {
	return "jwt"
}

type Base64Converter struct {
}

// RequiredConfigs implements Converter.
func (b *Base64Converter) RequiredConfigs() (string, string, []string, []string) {
	return "", "", nil, nil
}

// GetType implements Converter.
func (b *Base64Converter) GetType() ConvertKind {
	return base64Kind
}

// Convert implements Converter.
func (b *Base64Converter) Convert(source *Source) *Source {
	encoded := base64.StdEncoding.EncodeToString(source.content)
	return &Source{
		sourceType: TextType,
		content:    []byte(encoded),
	}
}

// GetName implements Converter.
func (b *Base64Converter) GetName() string {
	return "base64"
}

type Base64DecodeConverter struct {
}

// RequiredConfigs implements Converter.
func (b *Base64DecodeConverter) RequiredConfigs() (string, string, []string, []string) {
	return "", "", nil, nil
}

// GetType implements Converter.
func (b *Base64DecodeConverter) GetType() ConvertKind {
	return base64DecodeKind
}

// Convert implements Converter.
func (b *Base64DecodeConverter) Convert(source *Source) *Source {
	result, err := base64.StdEncoding.DecodeString(string(source.content))
	return &Source{
		writable: false,
		content:  result,
		err:      err,
	}
}

// GetName implements Converter.
func (b *Base64DecodeConverter) GetName() string {
	return "base64Decode"
}

type CommonNodeBase struct {
	clickable      widget.Clickable
	conversions    []*Conversion
	widList        widget.List
	discloserState component.DiscloserState
}

func (cnb *CommonNodeBase) GetDiscloserState() *component.DiscloserState {
	return &cnb.discloserState
}

func (cnb *CommonNodeBase) GetClickable() *widget.Clickable {
	return &cnb.clickable
}

func CreateConverter(kind ConvertKind) Converter {
	switch kind {
	case jwtKind:
		return &JwtConverter{}
	case base64Kind:
		return &Base64Converter{}
	case base64DecodeKind:
		return &Base64DecodeConverter{}
	}
	return nil
}

func NewConversion(src *Conversion, kind ConvertKind) *Conversion {
	c := &Conversion{
		origin: src,
	}
	c.widList.Axis = layout.Vertical
	c.converter = CreateConverter(kind)
	return c
}

func (cnb *CommonNodeBase) AddConversion(src *Conversion, kind ConvertKind) *Conversion {
	conv := NewConversion(src, kind)

	cnb.conversions = append(cnb.conversions, conv)

	logger.Info("Added a new convert", zap.String("name", conv.GetName()), zap.String("to", src.GetName()))
	return conv
}

type ConvertKind string

var (
	noneKind         ConvertKind = "none"
	jwtKind          ConvertKind = "jwt"
	base64Kind       ConvertKind = "base64"
	base64DecodeKind ConvertKind = "base64Decode"
)

type ConvertAction struct {
	name      string
	clickable widget.Clickable
	kind      ConvertKind
}

func (c *ConvertAction) GetOptionKeysAndValues() (string, string, []string, []string) {
	converter := CreateConverter(c.kind)
	if converter != nil {
		return converter.RequiredConfigs()
	}
	return "", "", nil, nil
}

func (c *ConvertAction) DoFor(gtx layout.Context, ct *ConvertTool, options map[string]string) {
	if ct.currentItem != nil {
		newconv := ct.currentItem.AddConversion(ct.currentItem, c.kind)
		ct.currentItem = newconv
	}
}

type ConvertTool struct {
	widget    layout.Widget
	clickable widget.Clickable

	newTargetBtnTooltip component.Tooltip
	newTargetClickable  widget.Clickable

	conversionTopBar widget.Editor
	sourceEditor     widget.Editor
	targetEditor     widget.Editor

	newTargetBtnTipArea component.TipArea

	targetArea      layout.Widget
	resize          component.Resize
	menuState       component.MenuState
	menuContextArea component.ContextArea
	actions         []*ConvertAction

	currentItem    *Conversion
	convList       []*Conversion
	convWidgetList widget.List

	//temp testing purpose
	optDialog  *common.OptionDialog
	showDialog bool
}

func NewItemName() string {
	currentTime := time.Now()
	return currentTime.Format("item-15:04:05.000")
}

type RootConverter struct {
	name string
}

// RequiredConfigs implements Converter.
func (r *RootConverter) RequiredConfigs() (string, string, []string, []string) {
	return "", "", nil, nil
}

func (r *RootConverter) Convert(source *Source) *Source {
	return nil
}

// GetName implements Converter.
func (r *RootConverter) GetName() string {
	return r.name
}

// GetType implements Converter.
func (r *RootConverter) GetType() ConvertKind {
	return noneKind
}

func NewRootConverter(name string) Converter {
	return &RootConverter{
		name: name,
	}
}

func NewRootConversion(initContent string) *Conversion {
	item0 := &Conversion{
		converter: NewRootConverter(NewItemName()),
		origin:    nil,
		convertResult: &Source{
			sourceType: TextType,
			err:        nil,
			content:    []byte(initContent),
			writable:   true,
		},
	}
	item0.widList.Axis = layout.Vertical
	return item0
}

func (c *ConvertTool) NewConversionItem() {
	c.convList = append(c.convList, NewRootConversion("Hello World!"))
}

func (c *ConvertTool) GetBarButtons(th *material.Theme) []layout.FlexChild {
	children := make([]layout.FlexChild, 0)
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(10)}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				if c.newTargetClickable.Clicked(gtx) {
					c.NewConversionItem()
				}
				button := component.TipIconButtonStyle{
					Tooltip:         c.newTargetBtnTooltip,
					IconButtonStyle: material.IconButton(th, &c.newTargetClickable, graphics.AddIcon, "New"),
					State:           &c.newTargetBtnTipArea,
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
func (c *ConvertTool) GetClickable() *widget.Clickable {
	return &c.clickable
}

// GetName implements Tool.
func (c *ConvertTool) GetName() string {
	return "convert"
}

func (c *ConvertTool) GetTabButtons(th *material.Theme) []layout.FlexChild {
	return []layout.FlexChild{}
}

func (c *ConvertTool) GetWidget() layout.Widget {
	return c.widget
}

func (c *ConvertTool) updateConversionPanel() {

	src := c.currentItem.GetSource()

	//the top item never have source.
	c.sourceEditor.ReadOnly = src != nil && !src.writable

	c.sourceEditor.SetText(c.currentItem.GetSourceContent())

	source := c.currentItem.GetSourceName()
	conv := c.currentItem.GetConvertKind()
	if conv == noneKind {
		c.conversionTopBar.SetText(c.currentItem.GetName())
		c.targetEditor.SetText("")
	} else {
		// todo: make conv a clickable to show conv config if any (like jwt)
		c.conversionTopBar.SetText(source + " → (" + string(conv) + ")")
		c.targetEditor.SetText(c.currentItem.GetValueAsString())
	}
}

func (c *ConvertTool) layoutConversion(th *material.Theme, gtx layout.Context, conv *Conversion) layout.Dimensions {

	if conv.clickable.Clicked(gtx) {
		if c.currentItem != conv {
			c.currentItem = conv
		}
		c.updateConversionPanel()
	}

	selected := c.currentItem == conv

	if len(conv.conversions) == 0 {
		return LeafClickableLabel(gtx, conv.GetClickable(), th, conv.GetName(), selected)
	}

	return component.SimpleDiscloser(th, &conv.discloserState).Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return ClickableLabel(gtx, conv.GetClickable(), th, conv.GetName(), selected)
		},
		func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &conv.widList).Layout(gtx, len(conv.conversions),
				func(gtx layout.Context, index int) layout.Dimensions {
					return c.layoutConversion(th, gtx, conv.conversions[index])
				})
		},
	)
}

func (c *ConvertTool) loadConversions() {
	c.convList = make([]*Conversion, 0)
}

func (c *ConvertTool) NewJwt() *ConvertAction {
	return &ConvertAction{
		name: "New jwt",
		kind: jwtKind,
	}
}

func (c *ConvertTool) NewBase64() *ConvertAction {
	return &ConvertAction{
		name: "New base64",
		kind: base64Kind,
	}
}

func (c *ConvertTool) NewBase64Decode() *ConvertAction {
	return &ConvertAction{
		name: "New base64decode",
		kind: base64DecodeKind,
	}
}

func (c *ConvertTool) initMenu(th *material.Theme) {

	convMenuItems := make([]func(gtx layout.Context) layout.Dimensions, 0)

	c.actions = make([]*ConvertAction, 0)
	c.actions = append(c.actions, c.NewBase64(), c.NewBase64Decode(), c.NewJwt())

	for _, a := range c.actions {
		convMenuItems = append(convMenuItems, component.MenuItem(th, &a.clickable, a.name).Layout)
	}

	c.menuState = component.MenuState{
		Options: convMenuItems,
	}
}

func NewConvertTool(th *material.Theme) Tool {
	c := &ConvertTool{}
	c.newTargetBtnTooltip = component.DesktopTooltip(th, "New")
	c.convWidgetList.Axis = layout.Vertical

	c.initMenu(th)

	//simulate loaded converions
	c.loadConversions()

	//testing, these belongs to converters
	c.optDialog = common.NewOptionDialog("", "", nil, nil, func(actionType common.ActionType, options map[string]string) {
		c.showDialog = false
	})

	c.targetArea = func(gtx layout.Context) layout.Dimensions {
		for _, a := range c.actions {
			if a.clickable.Clicked(gtx) {
				if title, subTitle, keys, defValues := a.GetOptionKeysAndValues(); len(keys) > 0 {

					c.optDialog.SetOptions(title, subTitle, keys, defValues)

					c.optDialog.SetCallback(func(actionType common.ActionType, options map[string]string) {
						c.showDialog = false
						if actionType == common.OK {
							a.DoFor(gtx, c, options)
						}
					})
					c.showDialog = true
				} else {
					a.DoFor(gtx, c, nil)
				}
			}
		}

		return layout.Stack{}.Layout(gtx,
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return material.List(th, &c.convWidgetList).Layout(gtx, len(c.convList), func(gtx layout.Context, index int) layout.Dimensions {
					item := c.convList[index]
					return c.layoutConversion(th, gtx, item)
				})
			}),
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				return c.menuContextArea.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = 0
					return component.Menu(th, &c.menuState).Layout(gtx)
				})
			}),
		)
	}
	c.resize.Ratio = 0.4

	leftPart := func(gtx layout.Context) layout.Dimensions {
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
			// the editor
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(10)).Layout(gtx, c.targetArea)
			}),
		)
	}

	c.conversionTopBar.SingleLine = true
	c.conversionTopBar.LineHeight = unit.Sp(18)
	c.conversionTopBar.LineHeightScale = 0.8
	c.conversionTopBar.ReadOnly = true

	// it has a top bar showing conversion source and target
	// below it the split: the left shows the source content
	// the right shows the converted content
	rightPart := func(gtx layout.Context) layout.Dimensions {

		editor := material.Editor(th, &c.conversionTopBar, "conversion")
		editor.Font.Weight = font.Bold

		c.targetEditor.ReadOnly = true
		sourceEditor := material.Editor(th, &c.sourceEditor, "source")
		targetEditor := material.Editor(th, &c.targetEditor, "target")

		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return editor.Layout(gtx)
			}),
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				changed := false
				if c.currentItem != nil && !c.currentItem.IsReadOnly() {
					for {
						evt, ok := c.sourceEditor.Update(gtx)
						if !ok {
							break
						}
						if _, isChange := evt.(widget.ChangeEvent); isChange {
							c.currentItem.UpdateContent(c.sourceEditor.Text())
							changed = true
						}
					}
				}

				if changed {
					if c.currentItem.origin != nil {
						c.currentItem.doConversion()
						c.targetEditor.SetText(c.currentItem.GetValueAsString())
					}
				}

				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
						return sourceEditor.Layout(gtx)
					}),
					layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
						return targetEditor.Layout(gtx)
					}),
				)
			}),
		)
	}

	c.widget = func(gtx layout.Context) layout.Dimensions {

		if c.showDialog {
			return c.optDialog.Layout(gtx, th)
		}

		return c.resize.Layout(gtx, leftPart, rightPart, common.VerticalSplitHandler)
	}
	return c
}

func NewToolsTab(th *material.Theme) *ToolsTab {
	tab := &ToolsTab{}

	tab.rigidButtons = make([]layout.FlexChild, 0)

	tab.clearBtnTooltip = component.DesktopTooltip(th, "Clear")

	clearBtn := component.TipIconButtonStyle{
		Tooltip:         tab.clearBtnTooltip,
		IconButtonStyle: material.IconButton(th, &tab.clearBtn, graphics.ClearIcon, "Clear log"),
		State:           &tab.clearBtnTipArea,
	}

	clearBtn.Size = 16
	clearBtn.IconButtonStyle.Inset = layout.Inset{Top: 1, Bottom: 1, Left: 1, Right: 1}

	rigid1 := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		if tab.clearBtn.Clicked(gtx) {
		}
		return layout.Inset{Top: 4, Bottom: 0, Left: 0, Right: 4}.Layout(gtx, clearBtn.Layout)
	})
	tab.rigidButtons = append(tab.rigidButtons, rigid1)

	tab.tools = make([]Tool, 0)

	tab.tools = append(tab.tools, NewConvertTool(th))

	if tab.currentTool == nil {
		tab.currentTool = tab.tools[0]
	}

	toolTabs := func(gtx layout.Context) layout.Dimensions {
		return tab.toolsList.Layout(gtx, len(tab.tools), func(gtx layout.Context, index int) layout.Dimensions {

			tool := tab.tools[index]
			if tool.GetClickable().Clicked(gtx) {
				tab.currentTool = tool
			}

			return material.Clickable(gtx, tool.GetClickable(), func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{
					Top:    unit.Dp(0),
					Bottom: unit.Dp(0),
					Left:   unit.Dp(10),
					Right:  unit.Dp(0),
				}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					label := material.H6(th, tool.GetName())
					if tab.currentTool == tool {
						label.Font.Weight = font.Bold
					}
					label.TextSize = unit.Sp(15)
					return layout.Inset{Top: 4, Bottom: 0, Left: 0, Right: 6}.Layout(gtx, label.Layout)
				})
			})

		})
	}

	toolsBar := make([]layout.FlexChild, 0)

	child0 := layout.Flexed(1.0, toolTabs)

	toolsBar = append(toolsBar, child0)

	//type should be layout.Rigid
	rigidButtons := tab.currentTool.GetTabButtons(th)

	toolsBar = append(toolsBar, rigidButtons...)

	tab.widget = func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					toolsBar...,
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				divider := component.Divider(th)
				divider.Top = unit.Dp(0)
				divider.Bottom = unit.Dp(0)
				divider.Fill = th.Palette.ContrastBg
				return divider.Layout(gtx)
			}),
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 5, Bottom: 0, Left: 5, Right: 0}.Layout(gtx,
					tab.currentTool.GetWidget())
			}),
		)
	}

	return tab
}

func LeafClickableLabel(gtx layout.Context, clickable *widget.Clickable, th *material.Theme, name string, selected bool) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max = image.Point{X: 16, Y: 16}
			color := common.COLOR.Gray()
			if selected {
				color = common.COLOR.Black()
			}
			return graphics.ResourceIcon.Layout(gtx, color)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Spacer{Width: unit.Dp(4)}.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Clickable(gtx, clickable, func(gtx layout.Context) layout.Dimensions {
				flatBtnText := material.Body1(th, name)
				if selected {
					flatBtnText.Font.Weight = font.Bold
				} else {
					flatBtnText.Font.Weight = font.Normal
				}
				return flatBtnText.Layout(gtx)
			})
		}),
	)
}

func ClickableLabel(gtx layout.Context, clickable *widget.Clickable, th *material.Theme, name string, selected bool) layout.Dimensions {
	return material.Clickable(gtx, clickable, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			flatBtnText := material.Body1(th, name)
			if selected {
				flatBtnText.Font.Weight = font.Bold
			} else {
				flatBtnText.Font.Weight = font.Normal
			}
			return layout.Center.Layout(gtx, flatBtnText.Layout)
		})
	})
}
