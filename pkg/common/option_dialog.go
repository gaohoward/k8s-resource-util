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
	"go.uber.org/zap"
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

type OptionDialog struct {
	title           string
	subTitle        string
	okClickable     widget.Clickable
	cancelClickable widget.Clickable
	keyValues       *om.OrderedMap[string, string]
	optionWidgets   []*OptionWidget
	optionsList     widget.List
	callback        DialogCallback
}

func (od *OptionDialog) SetOptions(title string, subTitle string, keys []string, defValues []string, desc []string) {
	od.title = title
	od.subTitle = subTitle

	diff := len(keys) - len(defValues)
	if diff > 0 {
		for range diff {
			defValues = append(defValues, "")
		}
	}

	od.optionWidgets = make([]*OptionWidget, len(keys))
	for i := range keys {
		od.keyValues.Set(keys[i], defValues[i])
		od.optionWidgets[i] = &OptionWidget{
			key:   keys[i],
			value: defValues[i],
			desc:  desc[i],
		}
		od.optionWidgets[i].valueField.Editor.SingleLine = true
		od.optionWidgets[i].valueField.Editor.SetText(defValues[i])
	}

}

type ActionType int

const (
	OK     ActionType = 0
	CANCEL ActionType = 1
)

type DialogCallback func(actionType ActionType, options map[string]string)

var NoOpCallback DialogCallback = func(actionType ActionType, options map[string]string) {
	logger.Info("Dialog submitted", zap.Int("action type", int(actionType)))
	for k, v := range options {
		logger.Info("collected a option", zap.String("key", k), zap.String("value", v))
	}
}

func NewOptionDialog(title string, subTitle string, keys []string, defValues []string, callback DialogCallback) *OptionDialog {
	od := &OptionDialog{
		title:     title,
		subTitle:  subTitle,
		keyValues: om.New[string, string](),
		callback:  callback,
	}

	od.optionsList.Axis = layout.Vertical

	diff := len(keys) - len(defValues)
	if diff > 0 {
		for range diff {
			defValues = append(defValues, "")
		}
	}

	od.optionWidgets = make([]*OptionWidget, len(keys))
	for i := range keys {
		od.keyValues.Set(keys[i], defValues[i])
		od.optionWidgets[i] = &OptionWidget{
			key:   keys[i],
			value: defValues[i],
		}
		od.optionWidgets[i].valueField.Editor.SingleLine = true
		od.optionWidgets[i].valueField.Editor.SetText(defValues[i])
	}
	return od
}

func (od *OptionDialog) CollectCurrentOptions() map[string]string {
	options := make(map[string]string, len(od.optionWidgets))
	for _, w := range od.optionWidgets {
		options[w.key] = w.valueField.Editor.Text()
	}
	return options
}

func (od *OptionDialog) SetCallback(cb DialogCallback) {
	od.callback = cb
}

func (od *OptionDialog) Layout(
	gtx layout.Context,
	th *material.Theme) layout.Dimensions {

	children := make([]layout.FlexChild, 5)

	// 1 title
	children[0] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		titleLb := material.Label(th, unit.Sp(24), od.title)
		titleLb.Font.Weight = font.Bold
		titleLb.Color = COLOR.Blue

		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
			layout.Rigid(titleLb.Layout),
		)
	})

	biggerOne := max(od.title, od.subTitle)

	size := GetAboutWidth(gtx, th, biggerOne)

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
		return material.Body1(th, od.subTitle).Layout(gtx)
	})

	// 4 options
	children[3] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		// measure the longest key label
		longestKey := od.getLongestKey()
		dims := MeasureLabelSize(gtx, th, longestKey)

		return material.List(th, &od.optionsList).Layout(gtx, len(od.optionWidgets), func(gtx layout.Context, index int) layout.Dimensions {
			option := od.optionWidgets[index]
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
	})

	// 5 buttons
	children[4] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		if od.okClickable.Clicked(gtx) {
			od.callback(OK, od.CollectCurrentOptions())
		}
		if od.cancelClickable.Clicked(gtx) {
			od.callback(CANCEL, od.CollectCurrentOptions())
		}

		return layout.Inset{Top: unit.Dp(20), Bottom: unit.Dp(0)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, material.Button(th, &od.cancelClickable, "Cancel").Layout)
				}),
				layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Center.Layout(gtx, material.Button(th, &od.okClickable, "Ok").Layout)
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

func (od *OptionDialog) getLongestKey() string {
	longest := ""
	for _, o := range od.optionWidgets {
		if len(o.key) > len(longest) {
			longest = o.key
		}
	}
	return longest
}

func MeasureWidgetSize(gtx layout.Context, th *material.Theme, widget layout.Widget) layout.Dimensions {
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

func MeasureTextFieldSize(gtx layout.Context, th *material.Theme, text string) layout.Dimensions {
	tf := component.TextField{}
	tf.Editor.SingleLine = true
	tf.SetText(text)
	return MeasureWidgetSize(gtx, th, func(gtx layout.Context) layout.Dimensions {
		return tf.Layout(gtx, th, text)
	})
}

func MeasureLabelSize(gtx layout.Context, th *material.Theme, labelStr string) layout.Dimensions {
	label := material.Label(th, unit.Sp(16), labelStr)
	label.Font.Weight = font.Bold
	label.TextSize = unit.Sp(16)
	label.Alignment = text.Start
	label.MaxLines = 1

	inset := layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2), Left: unit.Dp(2), Right: unit.Dp(4)}
	return MeasureWidgetSize(gtx, th, func(gtx layout.Context) layout.Dimensions {
		return inset.Layout(gtx, label.Layout)
	})
}
