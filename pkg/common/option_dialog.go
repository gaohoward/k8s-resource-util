package common

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	om "github.com/wk8/go-ordered-map/v2"
)

type OptionWidget struct {
	key        string
	valueField component.TextField
	value      string
	desc       string
}

func (o *OptionWidget) GetDescription() string {
	return o.desc
}

type DialogBase struct {
	title           string
	subTitle        string
	okClickable     widget.Clickable
	cancelClickable widget.Clickable
}

type TargetPart interface {
	Layout(gtx layout.Context, maxWidth int) layout.Dimensions
	Apply()
	Cancel()
}

func (db *DialogBase) Layout(
	gtx layout.Context,
	target TargetPart) layout.Dimensions {

	th := GetTheme()
	children := make([]layout.FlexChild, 5)

	// 1 title
	children[0] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		titleLb := material.Label(th, unit.Sp(24), db.title)
		titleLb.Font.Weight = font.Bold
		titleLb.Color = COLOR.Blue

		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
			layout.Rigid(titleLb.Layout),
		)
	})

	biggerOne := max(db.title, db.subTitle)

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
		return material.Body1(th, db.subTitle).Layout(gtx)
	})

	// 4 options
	children[3] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return target.Layout(gtx, size.Size.X)
	})

	// 5 buttons
	children[4] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		if db.okClickable.Clicked(gtx) {
			target.Apply()
		}
		if db.cancelClickable.Clicked(gtx) {
			target.Cancel()
		}

		return layout.Inset{Top: unit.Dp(20), Bottom: unit.Dp(0)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, material.Button(th, &db.cancelClickable, "Cancel").Layout)
				}),
				layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Center.Layout(gtx, material.Button(th, &db.okClickable, "Ok").Layout)
					})
				}),
			)
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

type EditDialog struct {
	dialogBase *DialogBase
	editTarget *EditorDialogTarget
}

func (od *EditDialog) SetContent(content string) {
	od.editTarget.editor.SetText(content)
}

func (od *EditDialog) SetCallback(f func(actionType ActionType, content string)) {
	od.editTarget.callback = f
}

func (od *EditDialog) SetTitle(title string) {
	od.dialogBase.title = title
}

type OptionDialog struct {
	dialogBase   *DialogBase
	optionTarget *OptionDialogTarget
}

func (ot *OptionDialogTarget) SetOptions(keys []string, defValues []string, desc []string) {

	diff := len(keys) - len(defValues)
	if diff > 0 {
		for range diff {
			defValues = append(defValues, "")
		}
	}

	ot.optionWidgets = make([]*OptionWidget, len(keys))
	for i := range keys {
		ot.keyValues.Set(keys[i], defValues[i])
		ot.optionWidgets[i] = &OptionWidget{
			key:   keys[i],
			value: defValues[i],
		}
		if desc != nil {
			ot.optionWidgets[i].desc = desc[i]
		} else {
			ot.optionWidgets[i].desc = keys[i]
		}
		ot.optionWidgets[i].valueField.Editor.SingleLine = true
		ot.optionWidgets[i].valueField.Editor.SetText(defValues[i])
	}
}

func (od *OptionDialog) SetOptions(title string, subTitle string, keys []string, defValues []string, desc []string) {
	od.dialogBase.title = title
	od.dialogBase.subTitle = subTitle

	diff := len(keys) - len(defValues)
	if diff > 0 {
		for range diff {
			defValues = append(defValues, "")
		}
	}

	od.optionTarget.SetOptions(keys, defValues, desc)
}

type ActionType int

const (
	OK     ActionType = 0
	CANCEL ActionType = 1
)

type OptionDialogCallback func(actionType ActionType, options map[string]string)
type EditDialogCallback func(actionType ActionType, content string)

func NewEditDialog(title string, subTitle string, initContent string, callback EditDialogCallback) *EditDialog {
	ed := &EditDialog{
		dialogBase: &DialogBase{
			title:    title,
			subTitle: subTitle,
		},
	}

	target := &EditorDialogTarget{
		callback: callback,
	}

	target.editorStyle = material.Editor(GetTheme(), &target.editor, "")
	target.editorStyle.Font.Typeface = "monospace"
	target.editorStyle.Font.Weight = font.Bold
	target.editorStyle.TextSize = unit.Sp(16)
	target.editorStyle.LineHeight = unit.Sp(16)

	ed.editTarget = target
	return ed
}

type EditorDialogTarget struct {
	callback    EditDialogCallback
	editorStyle material.EditorStyle
	editor      widget.Editor
}

// Apply implements [TargetPart].
func (e *EditorDialogTarget) Apply() {
	e.callback(OK, e.editor.Text())
}

// Cancel implements [TargetPart].
func (e *EditorDialogTarget) Cancel() {
	e.callback(CANCEL, e.editor.Text())
}

// Layout implements [TargetPart].
func (e *EditorDialogTarget) Layout(gtx layout.Context, width int) layout.Dimensions {
	border := widget.Border{
		Color:        COLOR.Blue,
		CornerRadius: unit.Dp(5),
		Width:        unit.Dp(2),
	}
	return border.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if gtx.Constraints.Min.X < width {
			gtx.Constraints.Min.X = width
		}
		if e.editor.LineHeight > 0 {
			gtx.Constraints.Min.Y = int(3 * e.editor.LineHeight)
		} else {
			gtx.Constraints.Min.Y = 48
		}
		return layout.Inset{Top: 2, Bottom: 2, Left: 2, Right: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return e.editorStyle.Layout(gtx)
		})
	})
}

func (od *EditDialog) Layout(
	gtx layout.Context) layout.Dimensions {
	return od.dialogBase.Layout(gtx, od.editTarget)
}

func NewOptionDialog(title string, subTitle string, keys []string, defValues []string, callback OptionDialogCallback) *OptionDialog {
	od := &OptionDialog{
		dialogBase: &DialogBase{
			title:    title,
			subTitle: subTitle,
		},
	}

	target := &OptionDialogTarget{
		keyValues: om.New[string, string](),
		callback:  callback,
	}

	target.optionsList.Axis = layout.Vertical

	target.SetOptions(keys, defValues, nil)

	od.optionTarget = target
	return od
}

func (od *OptionDialogTarget) CollectCurrentOptions() map[string]string {
	options := make(map[string]string, len(od.optionWidgets))
	for _, w := range od.optionWidgets {
		options[w.key] = w.valueField.Editor.Text()
	}
	return options
}

func (od *OptionDialog) SetCallback(cb OptionDialogCallback) {
	od.optionTarget.callback = cb
}

type OptionDialogTarget struct {
	keyValues     *om.OrderedMap[string, string]
	optionWidgets []*OptionWidget
	optionsList   widget.List
	callback      OptionDialogCallback
}

// Apply implements [TargetPart].
func (o *OptionDialogTarget) Apply() {
	o.callback(OK, o.CollectCurrentOptions())
}

// Cancel implements [TargetPart].
func (o *OptionDialogTarget) Cancel() {
	o.callback(CANCEL, o.CollectCurrentOptions())
}

// Layout implements [TargetPart].
func (o *OptionDialogTarget) Layout(gtx layout.Context, _ int) layout.Dimensions {
	th := GetTheme()
	// measure the longest key label
	longestKey := o.getLongestKey()
	dims := MeasureLabelSize(gtx, longestKey)

	return material.List(th, &o.optionsList).Layout(gtx, len(o.optionWidgets), func(gtx layout.Context, index int) layout.Dimensions {
		option := o.optionWidgets[index]
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				d := material.Label(th, unit.Sp(16), option.key).Layout(gtx)
				d.Size.X = dims.Size.X + gtx.Dp(unit.Dp(10))
				return d
			}),
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				return option.valueField.Layout(gtx, th, option.GetDescription())
			}),
		)
	})
}

func (od *OptionDialog) Layout(
	gtx layout.Context) layout.Dimensions {

	return od.dialogBase.Layout(gtx, od.optionTarget)
}

func (ot *OptionDialogTarget) getLongestKey() string {
	longest := ""
	for _, o := range ot.optionWidgets {
		if len(o.key) > len(longest) {
			longest = o.key
		}
	}
	return longest
}

func MeasureWidgetSize(gtx layout.Context, widget layout.Widget) layout.Dimensions {
	newGtx := layout.Context{
		Constraints: gtx.Constraints,
		Metric:      gtx.Metric,
		Now:         gtx.Now,
		Locale:      gtx.Locale,
		Source:      gtx.Source,
		Values:      gtx.Values,
		Ops:         &op.Ops{},
	}

	macro := op.Record(newGtx.Ops)
	dims := widget(newGtx)
	macro.Stop()
	return dims
}

func MeasureTextFieldSize(gtx layout.Context, text string) layout.Dimensions {
	tf := component.TextField{}
	tf.Editor.SingleLine = true
	tf.SetText(text)
	return MeasureWidgetSize(gtx, func(gtx layout.Context) layout.Dimensions {
		return tf.Layout(gtx, GetTheme(), text)
	})
}

func MeasureLabelSize(gtx layout.Context, labelStr string) layout.Dimensions {

	label := material.Label(GetTheme(), unit.Sp(16), labelStr)
	label.Font.Weight = font.Bold
	label.TextSize = unit.Sp(16)
	label.Alignment = text.Start
	label.MaxLines = 1

	inset := layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2), Left: unit.Dp(2), Right: unit.Dp(4)}
	return MeasureWidgetSize(gtx, func(gtx layout.Context) layout.Dimensions {
		return inset.Layout(gtx, label.Layout)
	})
}
