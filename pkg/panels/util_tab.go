package panels

import (

	// "crypto/rsa"

	"strings"

	"gaohoward.tools/k8s/resutil/pkg/common"
	"gaohoward.tools/k8s/resutil/pkg/graphics"
	"gaohoward.tools/k8s/resutil/pkg/k8sservice"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
)

var EMPTY_STRING = ""

type Tool interface {
	GetClickable() *widget.Clickable
	GetName() string
	GetTabButtons() []layout.FlexChild
	GetWidget() layout.Widget
}

type ToolsTab struct {
	toolsTab widget.Clickable

	clearBtn        widget.Clickable
	clearBtnTooltip component.Tooltip
	clearBtnTipArea component.TipArea

	rigidButtons []layout.FlexChild

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
func (p *ToolsTab) GetTabButtons() []layout.FlexChild {
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

type RawApiTool struct {
	kubeClient k8sservice.K8sService
	widget     layout.Widget
	clickable  widget.Clickable
	uriField   component.TextField
	runBtn     widget.Clickable
	result     *common.ReadOnlyEditor
}

func (rat *RawApiTool) Run() {
	if rep, err := rat.kubeClient.DoRawRequest(strings.TrimSpace(rat.uriField.Text())); err == nil {
		rat.result.SetText(&rep, nil)
	} else {
		errInfo := "Error: " + err.Error()
		rat.result.SetText(&errInfo, nil)
	}
}

func (rat *RawApiTool) GetClickable() *widget.Clickable {
	return &rat.clickable
}

func (rat *RawApiTool) GetName() string {
	return "raw-api"
}

func (rat *RawApiTool) GetTabButtons() []layout.FlexChild {
	return nil
}

func (rat *RawApiTool) GetWidget() layout.Widget {
	return rat.widget
}

func NewToolsTab(client k8sservice.K8sService) *ToolsTab {
	tab := &ToolsTab{}

	th := common.GetTheme()

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

	tab.tools = append(tab.tools, NewConvertTool(), NewRawApiTool(client))
	if rt, err := NewReaderTool(); err == nil {
		tab.tools = append(tab.tools, rt)
	}

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
	rigidButtons := tab.currentTool.GetTabButtons()

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

// The raw api tools allows users to access api resources
// in raw http format, i.e. the kubectl raw option, for example
// kubectl get --raw "/apis/subresources.kubevirt.io"
// or "/apis/subresources.kubevirt.io/v1/guestfs"
func NewRawApiTool(client k8sservice.K8sService) *RawApiTool {
	th := common.GetTheme()

	rt := &RawApiTool{
		kubeClient: client,
	}

	rt.result = common.NewReadOnlyEditor("result", 16, nil, nil, true)

	rt.uriField.SingleLine = true
	rt.uriField.Submit = true

	goBtn := material.Button(th, &rt.runBtn, "Go!")

	actionBar := func(gtx layout.Context) layout.Dimensions {

		if rt.runBtn.Clicked(gtx) {
			go rt.Run()
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				return rt.uriField.Layout(gtx, th, "URI:")
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(5), Right: unit.Dp(5)}.Layout(gtx, goBtn.Layout)
			}),
		)
	}

	resultArea := func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 6, Bottom: 0, Left: 0, Right: 0}.Layout(gtx, rt.result.Layout)
	}

	rt.widget = func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(actionBar),
			layout.Flexed(1.0, resultArea),
		)
	}
	return rt
}
