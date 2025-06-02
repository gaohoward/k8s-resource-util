package panels

import (
	"bytes"

	"gaohoward.tools/k8s/resutil/pkg/common"
	"gaohoward.tools/k8s/resutil/pkg/graphics"
	"gaohoward.tools/k8s/resutil/pkg/logs"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var logger *zap.Logger

func init() {
	logger, _ = logs.NewAppLogger("api-res-tab")
}

type ApiResourceItem interface {
	ToYaml() string
	GetClickable() *widget.Clickable
	GetSchema() string
}

// represents a single api resource
type ApiResourceNode struct {
	groupVersion  string
	nodeClickable widget.Clickable
	nodeData      *common.ApiResourceEntry
}

func (a *ApiResourceNode) GetSchema() string {
	return a.nodeData.Schema
}

// GetClickable implements ApiResourceItem.
func (a *ApiResourceNode) GetClickable() *widget.Clickable {
	return &a.nodeClickable
}

func (a *ApiResourceNode) ToYaml() string {

	writer := bytes.NewBufferString("")

	encoder := yaml.NewEncoder(writer)
	defer encoder.Close()
	err := encoder.Encode(a.nodeData.ApiRes)
	if err != nil {
		return "Error " + err.Error()
	}
	return writer.String()
}

// represents a api group
type ApiResourceGroup struct {
	groupClickable widget.Clickable
	groupData      *v1.APIResourceList
	groupVersion   string
	resources      []*ApiResourceNode
	treeState      component.DiscloserState
}

func (a *ApiResourceGroup) GetSchema() string {
	return "No schema for api groups"
}

// GetClickable implements ApiResourceItem.
func (a *ApiResourceGroup) GetClickable() *widget.Clickable {
	return &a.groupClickable
}

func (a *ApiResourceGroup) ToYaml() string {
	yml := "Group Version: " + a.groupVersion + "\nApi Path: "
	if a.groupVersion == "v1" {
		//core
		yml = yml + "/api/v1"
	} else {
		yml = yml + "/apis/" + a.groupVersion
	}
	return yml
}

type ApiResourcesTab struct {
	title        string
	tabClickable widget.Clickable

	refreshButton  widget.Clickable
	refreshTooltip component.Tooltip
	refreshTipArea component.TipArea

	showSchema   widget.Bool
	resList      widget.List
	buttons      []layout.FlexChild
	client       *common.K8sClient
	widget       layout.Widget
	allApis      []*ApiResourceGroup
	resize       component.Resize
	schemaResize component.Resize
	current      ApiResourceItem
	detailPage   widget.Editor
	schemaPage   widget.Editor
	detailEditor material.EditorStyle
	schemaEditor material.EditorStyle
	inAppLogger  *zap.Logger
}

// GetClickable implements PanelTab.
func (a *ApiResourcesTab) GetClickable() *widget.Clickable {
	return &a.tabClickable
}

// GetTabButtons implements PanelTab.
func (a *ApiResourcesTab) GetTabButtons(th *material.Theme) []layout.FlexChild {
	return a.buttons
}

// GetTitle implements PanelTab.
func (a *ApiResourcesTab) GetTitle() string {
	return a.title
}

// GetWidget implements PanelTab.
func (a *ApiResourcesTab) GetWidget() layout.Widget {
	return a.widget
}

func (a *ApiResourcesTab) processClick(item ApiResourceItem, gtx layout.Context) {
	if item.GetClickable().Clicked(gtx) {
		a.current = item
		a.detailPage.SetText(item.ToYaml())
		if s := item.GetSchema(); s != "" {
			a.schemaPage.SetText(item.GetSchema())
		} else {
			a.schemaPage.SetText("no schema available")
		}
	}
}

func (a *ApiResourcesTab) LayoutApiResources(th *material.Theme, gtx layout.Context, agr *ApiResourceGroup) layout.Dimensions {

	children := make([]layout.FlexChild, 0)
	for _, n := range agr.resources {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			flatBtnText := material.Body1(th, n.nodeData.ApiRes.Name)
			flatBtnText.TextSize = unit.Sp(15)
			a.processClick(n, gtx)
			if a.current == n {
				flatBtnText.Font.Weight = font.Bold
			} else {
				flatBtnText.Font.Weight = font.Normal
			}
			return material.Clickable(gtx, &n.nodeClickable, func(gtx layout.Context) layout.Dimensions {
				return layout.W.Layout(gtx, flatBtnText.Layout)
			})
		}))
	}
	return component.SimpleDiscloser(th, &agr.treeState).Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			label := material.H6(th, agr.groupVersion)
			label.TextSize = 16
			a.processClick(agr, gtx)
			if a.current == agr {
				label.Font.Weight = font.Bold
			} else {
				label.Font.Weight = font.Normal
			}
			return material.Clickable(gtx, &agr.groupClickable, func(gtx layout.Context) layout.Dimensions {
				return label.Layout(gtx)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		},
	)
}

func (t *ApiResourcesTab) populateTableContents(resInfo *common.ApiResourceInfo, refresh bool) {
	if len(t.allApis) == 0 || refresh {

		t.allApis = make([]*ApiResourceGroup, 0)
		for _, gr := range resInfo.ResList {
			group := &ApiResourceGroup{
				groupData:    gr,
				groupVersion: gr.GroupVersion,
				resources:    make([]*ApiResourceNode, 0),
			}
			for _, rs := range gr.APIResources {
				node := &ApiResourceNode{
					groupVersion: gr.GroupVersion,
				}
				apiVer := gr.GroupVersion + "/" + rs.Name
				if r := resInfo.FindApiResource(apiVer); r != nil {
					node.nodeData = r
				} else {
					node.nodeData = &common.ApiResourceEntry{
						ApiVer: apiVer,
						Gv:     gr.GroupVersion,
						ApiRes: &rs,
						Schema: "No schema found",
					}
				}

				group.resources = append(group.resources, node)
			}
			t.allApis = append(t.allApis, group)
		}
	}
}

func NewApiResourcesTab(th *material.Theme, client *common.K8sClient) *ApiResourcesTab {

	tab := &ApiResourcesTab{
		title:   "api-resources",
		client:  client,
		allApis: make([]*ApiResourceGroup, 0),
		buttons: make([]layout.FlexChild, 0),
	}

	tab.inAppLogger = logs.GetLogger(logs.IN_APP_LOGGER_NAME)

	schemaCheck := material.CheckBox(th, &tab.showSchema, "Schema")
	schemaCheck.Size = 16
	rigid0 := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 4, Bottom: 0, Left: 0, Right: 4}.Layout(gtx, schemaCheck.Layout)
	})

	tab.buttons = append(tab.buttons, rigid0)

	tab.refreshTooltip = component.DesktopTooltip(th, "Reload")

	reloadBtn := component.TipIconButtonStyle{
		Tooltip:         tab.refreshTooltip,
		IconButtonStyle: material.IconButton(th, &tab.refreshButton, graphics.RefreshIcon, "Reload"),
		State:           &tab.refreshTipArea,
	}

	reloadBtn.Size = 16
	reloadBtn.IconButtonStyle.Inset = layout.Inset{Top: 1, Bottom: 1, Left: 1, Right: 1}

	rigid1 := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		if tab.refreshButton.Clicked(gtx) {
			newRes := tab.client.FetchAllApiResources(true)
			if newRes != nil {
				tab.populateTableContents(newRes, true)
				tab.inAppLogger.Info("Reloaded api-resources", zap.Int("total groups", len(tab.allApis)))
			}
		}
		return layout.Inset{Top: 4, Bottom: 0, Left: 0, Right: 4}.Layout(gtx, reloadBtn.Layout)
	})

	tab.buttons = append(tab.buttons, rigid1)

	tab.resList.Axis = layout.Vertical
	tab.resize.Ratio = 0.4

	tab.schemaResize.Ratio = 0.4

	apiResources := tab.client.FetchAllApiResources(true)

	if apiResources != nil {
		tab.populateTableContents(apiResources, false)
	}

	tab.detailEditor = material.Editor(th, &tab.detailPage, "details")
	tab.detailEditor.Font.Typeface = "monospace"
	//	tab.detailEditor.Font.Weight = font.Bold
	tab.detailEditor.TextSize = unit.Sp(15)
	tab.detailEditor.LineHeight = unit.Sp(15)
	if tab.current != nil {
		tab.detailPage.SetText(tab.current.ToYaml())
	}

	tab.schemaEditor = material.Editor(th, &tab.schemaPage, "schema")
	tab.schemaEditor.Font.Typeface = "monospace"
	//	tab.schemaEditor.Font.Weight = font.Bold
	tab.schemaEditor.TextSize = unit.Sp(15)
	tab.schemaEditor.LineHeight = unit.Sp(15)
	if tab.current != nil {
		if tab.current.GetSchema() != "" {
			tab.schemaPage.SetText(tab.current.GetSchema())
		} else {
			tab.schemaPage.SetText("no schema available")
		}
	}

	tab.widget = func(gtx layout.Context) layout.Dimensions {
		if tab.showSchema.Pressed() {
			tab.showSchema.Update(gtx)
		}
		if tab.showSchema.Value {
			return tab.resize.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return material.List(th, &tab.resList).Layout(gtx, len(tab.allApis),
						func(gtx layout.Context, index int) layout.Dimensions {
							agrp := tab.allApis[index]
							return layout.UniformInset(unit.Dp(4)).Layout(gtx,
								func(gtx layout.Context) layout.Dimensions {
									return tab.LayoutApiResources(th, gtx, agrp)
								})
						})
				},
				func(gtx layout.Context) layout.Dimensions {
					return tab.schemaResize.Layout(gtx,
						func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Top: 10, Bottom: 2, Left: 10, Right: 2}.Layout(gtx,
								func(gtx layout.Context) layout.Dimensions {
									return tab.detailEditor.Layout(gtx)
								})
						},
						func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Top: 10, Bottom: 2, Left: 10, Right: 2}.Layout(gtx,
								func(gtx layout.Context) layout.Dimensions {
									return tab.schemaEditor.Layout(gtx)
								})
						},
						func(gtx layout.Context) layout.Dimensions {
							return common.VerticalSplitHandler(gtx)
						},
					)
				},
				func(gtx layout.Context) layout.Dimensions {
					return common.VerticalSplitHandler(gtx)
				},
			)
		} else {
			return tab.resize.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return material.List(th, &tab.resList).Layout(gtx, len(tab.allApis),
						func(gtx layout.Context, index int) layout.Dimensions {
							agrp := tab.allApis[index]
							return layout.UniformInset(unit.Dp(4)).Layout(gtx,
								func(gtx layout.Context) layout.Dimensions {
									return tab.LayoutApiResources(th, gtx, agrp)
								})
						})
				},
				func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 10, Bottom: 2, Left: 10, Right: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return tab.detailEditor.Layout(gtx)
					})
				},
				func(gtx layout.Context) layout.Dimensions {
					return common.VerticalSplitHandler(gtx)
				},
			)
		}
	}
	return tab
}
