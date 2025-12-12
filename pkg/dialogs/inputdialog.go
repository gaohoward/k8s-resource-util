package dialogs

import (
	"image/color"

	"gaohoward.tools/k8s/resutil/pkg/logs"
	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"go.uber.org/zap"
)

var logger *zap.Logger

func init() {
	logger, _ = logs.NewAppLogger("dialog")
}

type DialogControl interface {
	GetWidget(th *material.Theme) layout.Widget
	Apply() error
	Cancel()
}

// Gio doesnt have a dialog yet
// we use modalsheet instead
type InputDialog struct {
	Title     string
	mainPanel DialogControl
	infoBar   material.LabelStyle
	// The ModalSheet is a side panel
	// meaning it layouts from the side of the window
	sheet              *component.ModalSheet
	layer              *component.ModalLayer
	confirm            widget.Clickable
	cancel             widget.Clickable
	submitBtnsOffset   unit.Dp
	submitBtnsInterval unit.Dp

	errorMessage string
}

func (dlg *InputDialog) SetError(err error) {
	dlg.errorMessage = err.Error()
}

func (dlg *InputDialog) SetControl(control DialogControl) {
	dlg.mainPanel = control
}

func NewInputDialog(title string, control DialogControl) *InputDialog {
	dialog := &InputDialog{Title: title}
	dialog.layer = component.NewModal()

	dialog.sheet = component.NewModalSheet(dialog.layer)
	dialog.sheet.Modal.FinalAlpha = 120
	dialog.sheet.MaxWidth = 500
	dialog.mainPanel = control
	dialog.submitBtnsOffset = 140
	dialog.submitBtnsInterval = 40

	dialog.sheet.LayoutModal(func(gtx layout.Context, th *material.Theme, anim *component.VisibilityAnimation) layout.Dimensions {

		for {
			_, ok := gtx.Event(key.Filter{
				Name: key.NameReturn,
			})
			if ok {
				// same as if apply button is pressed
				if err := dialog.mainPanel.Apply(); err != nil {
					dialog.SetError(err)
				} else {
					dialog.sheet.Disappear(gtx.Now)
				}
			} else {
				break
			}
		}

		dialog.infoBar = material.Label(th, unit.Sp(16), dialog.errorMessage)
		dialog.infoBar.Color = color.NRGBA{R: 255, G: 0, B: 0, A: 255} // Set color to red

		if dialog.confirm.Clicked(gtx) {
			if err := dialog.mainPanel.Apply(); err != nil {
				dialog.SetError(err)
			} else {
				dialog.sheet.Disappear(gtx.Now)
			}
		}
		if dialog.cancel.Clicked(gtx) {
			dialog.mainPanel.Cancel()
			dialog.sheet.Disappear(gtx.Now)
		}

		//Overrall layout uses Flex vertical
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			//title
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label := material.H6(th, dialog.Title)
				label.Font.Weight = font.Bold
				return layout.Inset{Top: unit.Dp(12)}.Layout(gtx, label.Layout)
			}),
			//separator
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				divider := component.Divider(th)
				divider.Top = unit.Dp(0)
				divider.Bottom = unit.Dp(0)
				divider.Thickness = unit.Dp(1)
				divider.Fill = th.Palette.ContrastBg
				return divider.Layout(gtx)
			}),
			//info bar
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return dialog.infoBar.Layout(gtx)
				// barWidth := gtx.Constraints.Max.X
				// barHeight := gtx.Dp(unit.Dp(24))
				// barRect := image.Rect(0, 0, barWidth, barHeight)
				// barColor := color.NRGBA{R: 224, G: 224, B: 224, A: 255}
				// paint.FillShape(gtx.Ops, barColor, clip.Rect(barRect).Op())
				// return layout.Dimensions{
				// 	Size: image.Point{X: barWidth, Y: barHeight},
				// }
			}),
			//the main widget
			layout.Flexed(1, dialog.mainPanel.GetWidget(th)),

			// submit buttons
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				btn1 := material.Button(th, &dialog.cancel, "Cancel")
				btn2 := material.Button(th, &dialog.confirm, "Confirm")
				return layout.Inset{Top: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: dialog.submitBtnsOffset, Bottom: 12}.Layout(gtx, btn1.Layout)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: dialog.submitBtnsInterval, Bottom: 12}.Layout(gtx, btn2.Layout)
						}),
					)
				})
			}),
		)
	})
	return dialog
}

func (dlg *InputDialog) Layout(gtx layout.Context, th *material.Theme) {
	dlg.sheet.Modal.Layout(gtx, th)
}

func (dlg *InputDialog) Show(gtx layout.Context) {
	dlg.sheet.Appear(gtx.Now)
}

var dialogRegistry []*InputDialog = make([]*InputDialog, 0)

func RegisterDialog(d *InputDialog) {
	dialogRegistry = append(dialogRegistry, d)
}

func LayoutDialogs(gtx layout.Context, th *material.Theme) {
	for _, d := range dialogRegistry {
		d.Layout(gtx, th)
	}
}
