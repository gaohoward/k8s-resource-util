package panels

import (
	"image"

	"gaohoward.tools/k8s/resutil/pkg/common"
	"gaohoward.tools/k8s/resutil/pkg/graphics"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
)

type DeploymentTab struct {
	tabButton          widget.Clickable
	undeployBtn        widget.Clickable
	undeployBtnTooltip component.Tooltip
	undeployBtnTipArea component.TipArea

	buttons  []layout.FlexChild
	widget   layout.Widget
	grid     component.GridState
	deployed *common.DeployedResources
	client   *common.K8sClient
	resMgr   common.ResourceManager
}

var headingText = []string{"", "Type", "Name", "Namespace", "State", "Creation"}

func (d *DeploymentTab) Load() {
	persister := d.deployed.GetPersister()
	loaded, _ := persister.Load()
	for _, itm := range loaded {
		if n, ok := d.resMgr.GetNodeMap()[itm.Id]; ok {
			itm.RestoreNode(n)
		} else {
			itm.SetOrphaned()
		}
		d.deployed.AddDetail(itm, false)
	}
}

// GetClickable implements PanelTab.
func (d *DeploymentTab) GetClickable() *widget.Clickable {
	return &d.tabButton
}

// GetTabButtons implements PanelTab.
func (d *DeploymentTab) GetTabButtons(th *material.Theme) []layout.FlexChild {
	return d.buttons
}

// GetTitle implements PanelTab.
func (d *DeploymentTab) GetTitle() string {
	return "deployments"
}

// GetWidget implements PanelTab.
func (d *DeploymentTab) GetWidget() layout.Widget {
	return d.widget
}

func NewDeploymentTab(th *material.Theme, dr *common.DeployedResources, k8sClient *common.K8sClient, resManager common.ResourceManager) *DeploymentTab {
	tab := &DeploymentTab{
		buttons:            make([]layout.FlexChild, 0),
		deployed:           dr,
		client:             k8sClient,
		resMgr:             resManager,
		undeployBtnTooltip: component.DesktopTooltip(th, "Undeploy"),
	}

	clearBtn := component.TipIconButtonStyle{
		Tooltip:         tab.undeployBtnTooltip,
		IconButtonStyle: material.IconButton(th, &tab.undeployBtn, graphics.UndeployIcon, "UnDeploy"),
		State:           &tab.undeployBtnTipArea,
	}

	clearBtn.Size = 16
	clearBtn.IconButtonStyle.Inset = layout.Inset{Top: 1, Bottom: 1, Left: 1, Right: 1}

	rigid1 := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		if tab.undeployBtn.Clicked(gtx) {
			ctxData, _ := common.GetContextData(common.CONTEXT_LONG_TASK_LIST)
			if taskCtx, ok := ctxData.(*common.LongTasksContext); ok {
				task := taskCtx.AddTask("Undeploying resource")
				task.Run = func() {
					var anyFailure error
					task.Progress = float32(0.1)
					len := len(dr.GetSelectedDeployments())
					task.Step = 0.9 / float32(len)
					for _, selected := range dr.GetSelectedDeployments() {
						for _, inst := range selected.AllInstances {
							inst.SetAction(common.Delete)
							targetNs := selected.OriginalCrs[inst.Instance.GetId()].FinalNs
							if _, err := tab.client.DeployResource(inst, targetNs); err != nil {
								anyFailure = err
								task.Update("Failed to undeploy " + inst.GetName() + " err: " + err.Error())
							} else {
								//update progress
								task.Update("Undeployed " + inst.GetName())
							}
						}
						dr.Remove(selected.Id)
					}
					if anyFailure != nil {
						task.Failed(anyFailure)
					} else {
						task.Done()
					}
				}
				task.Start()
			}
		}

		if !tab.deployed.AnySelected() {
			gtx = gtx.Disabled()
		}

		dims := layout.Inset{Top: 4, Bottom: 0, Left: 0, Right: 4}.Layout(gtx, clearBtn.Layout)

		return dims
	})

	tab.buttons = append(tab.buttons, rigid1)

	allRes := k8sClient.FetchAllApiResources(false)

	tab.widget = func(gtx layout.Context) layout.Dimensions {

		inset := layout.UniformInset(unit.Dp(2))

		// Configure a label styled to be a heading.
		headingLabel := material.Body1(th, "")
		headingLabel.Font.Weight = font.Bold
		headingLabel.TextSize = unit.Sp(14)
		headingLabel.Alignment = text.Start
		headingLabel.MaxLines = 1

		// Measure the height of a heading row.
		orig := gtx.Constraints
		gtx.Constraints.Min = image.Point{}
		dims := inset.Layout(gtx, headingLabel.Layout)
		gtx.Constraints = orig

		//5 columns: Checkbox, Name, Kind, Namespace, Status
		return component.Table(th, &tab.grid).Layout(gtx, tab.deployed.Size(), 6,
			func(axis layout.Axis, index, constraint int) int {
				switch axis {
				case layout.Horizontal:
					switch index {
					case 0:
						return int(26)
					case 1, 2, 3, 4, 5:
						return int(constraint / 5)
					default:
						return 0
					}
				default:
					return dims.Size.Y + 2
				}
			},
			func(gtx layout.Context, col int) layout.Dimensions {
				return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					headingLabel.Text = headingText[col]
					return headingLabel.Layout(gtx)
				})
			},
			func(gtx layout.Context, row, col int) layout.Dimensions {
				return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					dd := tab.deployed.Get(row)
					value := ""
					switch col {
					case 0:
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							if dd.GetCheckStatus().Pressed() {
								dd.GetCheckStatus().Update(gtx)
								common.GetAppWindow().Invalidate()
							}
							cb := material.CheckBox(th, dd.GetCheckStatus(), "")
							cb.Size = unit.Dp(15)
							return cb.Layout(gtx)
						})
					case 1:
						if dd.ApiVer == "" {
							value = "collection"
						} else {
							apiRes := allRes.FindApiResource(dd.ApiVer)
							if apiRes != nil {
								value = apiRes.ApiRes.Kind
							} else {
								value = "Not Found " + dd.ApiVer
							}
						}
					case 2:
						value = dd.Name
					case 3:
						value = dd.GetAllDeployNamespaces()
					case 4:
						value = dd.Status.String()
					case 5:
						value = dd.Creation
					}
					lb := material.Label(th, unit.Sp(15), value)
					return lb.Layout(gtx)
				})
			},
		)
	}
	tab.Load()
	return tab
}
