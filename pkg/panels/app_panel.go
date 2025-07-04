package panels

import (
	"gaohoward.tools/k8s/resutil/pkg/common"
	"gaohoward.tools/k8s/resutil/pkg/k8sservice"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
)

// A PanelTab represents a tab in the panel
type PanelTab interface {
	GetClickable() *widget.Clickable
	GetTitle() string
	GetTabButtons(th *material.Theme) []layout.FlexChild
	GetWidget() layout.Widget
}

type AppPanel struct {
	widget  layout.Widget
	tabList layout.List

	allTabs    []PanelTab
	currentTab PanelTab
}

func (p *AppPanel) GetWidget() layout.Widget {
	return p.widget
}

// each time it is called a new panel is created
// so far it should be only called once.
func GetAppPanel(th *material.Theme, dr *k8sservice.DeployedResources, k8sClient k8sservice.K8sService, resMgr common.ResourceManager) *AppPanel {
	panel := AppPanel{}

	depTab := NewDeploymentTab(th, dr, k8sClient, resMgr)
	common.SetContextData(common.CONTEXT_APP_INIT_STATE, float32(0.65), nil)
	logTab := NewLogTab(th)
	common.SetContextData(common.CONTEXT_APP_INIT_STATE, float32(0.7), nil)
	apiTab := NewApiResourcesTab(th, k8sClient)
	common.SetContextData(common.CONTEXT_APP_INIT_STATE, float32(0.75), nil)
	inKTab := NewInKubeTab(th, k8sClient)
	common.SetContextData(common.CONTEXT_APP_INIT_STATE, float32(0.8), nil)

	panel.allTabs = append(panel.allTabs,
		depTab,
		logTab,
		apiTab,
		inKTab)

	panel.widget = func(gtx layout.Context) layout.Dimensions {

		if panel.currentTab == nil {
			panel.currentTab = panel.allTabs[0]
		}

		panelTabs := func(gtx layout.Context) layout.Dimensions {
			return panel.tabList.Layout(gtx, len(panel.allTabs), func(gtx layout.Context, index int) layout.Dimensions {

				tab := panel.allTabs[index]
				if tab.GetClickable().Clicked(gtx) {
					panel.currentTab = tab
				}
				return material.Clickable(gtx, tab.GetClickable(), func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{
						Top:    unit.Dp(0),
						Bottom: unit.Dp(0),
						Left:   unit.Dp(10),
						Right:  unit.Dp(0),
					}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						label := material.H6(th, tab.GetTitle())
						if panel.currentTab == tab {
							label.Font.Weight = font.Bold
						}
						label.TextSize = unit.Sp(15)
						return layout.Inset{Top: 4, Bottom: 0, Left: 0, Right: 6}.Layout(gtx, label.Layout)
					})
				})
			})
		}

		tabBar := make([]layout.FlexChild, 0)

		child0 := layout.Flexed(1.0, panelTabs)

		tabBar = append(tabBar, child0)

		//type should be layout.Rigid
		rigidButtons := panel.currentTab.GetTabButtons(th)

		tabBar = append(tabBar, rigidButtons...)

		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					tabBar...,
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
					//panel.logEditor.Layout)
					panel.currentTab.GetWidget())
			}),
		)
	}
	return &panel
}
