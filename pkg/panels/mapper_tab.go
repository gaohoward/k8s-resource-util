package panels

import (
	"strings"

	"gaohoward.tools/k8s/resutil/pkg/common"
	"gaohoward.tools/k8s/resutil/pkg/graphics"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	om "github.com/wk8/go-ordered-map/v2"
)

// represents a single api resource
type KeyNode struct {
	name          string
	nodeClickable widget.Clickable
	nodeData      []string //values by key
}

func (a *KeyNode) Text() string {
	return strings.Join(a.nodeData, "\n")
}

// GetClickable implements ApiResourceItem.
func (a *KeyNode) GetClickable() *widget.Clickable {
	return &a.nodeClickable
}

func GetStaticMapper() *om.OrderedMap[string, []*KeyNode] {

	cdiControllerNodes := make([]*KeyNode, 0)

	ctrl1 := &KeyNode{
		name:     "clone-populator",
		nodeData: []string{"PVC", "VolumeSnapshot", "VolumeCloneSource", "Pod"},
	}

	ctrl2 := &KeyNode{
		name:     "upload-populator",
		nodeData: []string{"PVC", "VolumeUploadSource"},
	}

	ctrl3 := &KeyNode{
		name:     "import-populator",
		nodeData: []string{"PVC", "VolumeImportSource", "Event"},
	}

	ctrl4 := &KeyNode{
		name:     "datasource-controller",
		nodeData: []string{"DataVolume", "PVC", "VolumeSnapshot", "DataSource"},
	}

	ctrl5 := &KeyNode{
		name:     "import-controller",
		nodeData: []string{"PVC", "Pod"},
	}

	ctrl6 := &KeyNode{
		name:     "clone-controller",
		nodeData: []string{"PVC"},
	}

	ctrl7 := &KeyNode{
		name:     "upload-controller",
		nodeData: []string{"PVC", "Pod", "Service"},
	}

	ctrl8 := &KeyNode{
		name:     "objecttransfer-controller",
		nodeData: []string{"DataVolume", "PVC", "PV", "ObjectTransfer"},
	}

	ctrl9 := &KeyNode{
		name:     "dv-import-controller",
		nodeData: []string{"DataVolume", "PVC", "Pod", "PV", "ObjectTransfer", "VolumeImportSource", "StorageClass", "StorageProfile"},
	}

	ctrl10 := &KeyNode{
		name:     "dv-upload-controller",
		nodeData: []string{"DataVolume", "PVC", "Pod", "PV", "VolumeUploadSource", "StorageClass", "StorageProfile"},
	}

	ctrl11 := &KeyNode{
		name:     "dv-pvc-clone-controller",
		nodeData: []string{"DataVolume", "PVC", "VolumeCloneSource", "Pod", "PV", "DataSource", "StorageClass", "StorageProfile"},
	}

	ctrl12 := &KeyNode{
		name:     "dv-snapshot-clone-controller",
		nodeData: []string{"DataVolume", "PVC", "VolumeSnapshot", "VolumeCloneSource", "Pod", "PV", "ObjectTransfer", "DataSource", "StorageClass", "StorageProfile"},
	}

	ctrl13 := &KeyNode{
		name:     "dv-populator-controller",
		nodeData: []string{"DataVolume", "PVC", "Pod", "PV", "ObjectTransfer", "StorageClass", "StorageProfile"},
	}

	ctrl14 := &KeyNode{
		name:     "forklift-populator",
		nodeData: []string{"PVC", "Pod", "OvirtVolumePopulator", "OpenstackVolumePopulator"},
	}

	ctrl15 := &KeyNode{
		name:     "config-controller",
		nodeData: []string{"ConfigMap", "Ingress", "StorageClass", "Route", "ocpv1.Proxy", "CDIConfig", "CDI"},
	}

	ctrl16 := &KeyNode{
		name:     "dataimport-cron-controller",
		nodeData: []string{"DataVolume", "PVC", "VolumeSnapshot", "DataSource", "CronJob", "DataImportCron", "StorageClass", "StorageProfile"},
	}

	ctrl17 := &KeyNode{
		name:     "storage-profile-controller",
		nodeData: []string{"VolumeSnapshotClass", "PV", "StorageClass", "StorageProfile"},
	}

	cdiControllerNodes = append(cdiControllerNodes, ctrl1, ctrl2, ctrl3, ctrl4, ctrl5, ctrl6, ctrl7, ctrl8, ctrl9, ctrl10, ctrl11, ctrl12, ctrl13, ctrl14, ctrl15, ctrl16, ctrl17)

	mapperInfo := om.New[string, []*KeyNode]()

	mapperInfo.Set("controllers", cdiControllerNodes)

	resourceNodes := make([]*KeyNode, 0)

	res1 := &KeyNode{
		name:     "DataVolume",
		nodeData: []string{"datasource-controller", "object-transfer-controller", "dv-import-controller", "dv-upload-controller", "dv-pvc-clone-controller", "dv-snapshot-clone-controller", "dv-populator-controller", "data-import-cron-controller"},
	}

	res2 := &KeyNode{
		name:     "PVC",
		nodeData: []string{"clone-populator", "upload-populator", "import-populator", "datasource-controller", "import-controller", "clone-controller", "upload-controller", "object-transfer-controller", "dv-import-controller", "dv-upload-controller", "dv-pvc-clone-controller", "dv-snapshot-clone-controller", "dv-populator-controller", "forklift-populator", "data-import-cron-controller"},
	}

	res3 := &KeyNode{
		name:     "VolumeSnapshot",
		nodeData: []string{"clone-populator", "datasource-controller", "dv-snapshot-clone-controller", "data-import-cron-controller"},
	}

	res4 := &KeyNode{
		name:     "VolumeCloneSource",
		nodeData: []string{"clone-populator", "dv-pvc-clone-controller", "dv-snapshot-clone-controller"},
	}

	res5 := &KeyNode{
		name:     "Pod",
		nodeData: []string{"clone-populator", "import-controller", "upload-controller", "dv-import-controller", "dv-upload-controller", "dv-pvc-clone-controller", "dv-snapshot-clone-controller", "dv-populator-controller", "forklift-populator"},
	}

	res6 := &KeyNode{
		name:     "VolumeSnapshotClass",
		nodeData: []string{"storage-profile-controller"},
	}

	res7 := &KeyNode{
		name:     "PV",
		nodeData: []string{"object-transfer-controller", "dv-import-controller", "dv-upload-controller", "dv-pvc-clone-controller", "dv-snapshot-clone-controller", "dv-populator-controller", "storage-profile-controller"},
	}

	res8 := &KeyNode{
		name:     "ObjectTransfer",
		nodeData: []string{"object-transfer-controller", "dv-import-controller", "dv-upload-controller", "dv-pvc-clone-controller", "dv-snapshot-clone-controller", "dv-populator-controller"},
	}

	res9 := &KeyNode{
		name:     "VolumeImportSource",
		nodeData: []string{"import-populator", "dv-import-controller", "dv-upload-controller", "dv-pvc-clone-controller", "dv-snapshot-clone-controller", "dv-populator-controller", "forklift-populator", "data-import-cron-controller"},
	}

	res10 := &KeyNode{
		name:     "VolumeUploadSource",
		nodeData: []string{"upload-populator", "dv-upload-controller", "dv-pvc-clone-controller", "dv-snapshot-clone-controller", "dv-populator-controller", "forklift-populator", "data-import-cron-controller"},
	}

	res11 := &KeyNode{
		name:     "DataSource",
		nodeData: []string{"datasource-controller", "dv-pvc-clone-controller", "dv-snapshot-clone-controller", "data-import-cron-controller"},
	}

	res12 := &KeyNode{
		name:     "Service",
		nodeData: []string{"upload-controller"},
	}

	res13 := &KeyNode{
		name:     "ConfigMap",
		nodeData: []string{"config-controller"},
	}

	res14 := &KeyNode{
		name:     "CronJob",
		nodeData: []string{"data-import-cron-controller"},
	}

	res15 := &KeyNode{
		name:     "DataImportCron",
		nodeData: []string{"data-import-cron-controller"},
	}

	res16 := &KeyNode{
		name:     "Ingress",
		nodeData: []string{"config-controller"},
	}

	res17 := &KeyNode{
		name:     "StorageClass",
		nodeData: []string{"dv-import-controller", "dv-upload-controller", "dv-pvc-clone-controller", "dv-snapshot-clone-controller", "dv-populator-controller", "config-controller", "data-import-cron-controller", "storage-profile-controller"},
	}

	res18 := &KeyNode{
		name:     "Event",
		nodeData: []string{"import-populator"},
	}

	res19 := &KeyNode{
		name:     "Route",
		nodeData: []string{"config-controller"},
	}

	res20 := &KeyNode{
		name:     "ocpv1.Proxy",
		nodeData: []string{"config-controller"},
	}

	res21 := &KeyNode{
		name:     "CDIConfig",
		nodeData: []string{"config-controller"},
	}

	res22 := &KeyNode{
		name:     "CDI",
		nodeData: []string{"config-controller"},
	}

	res23 := &KeyNode{
		name:     "StorageProfile",
		nodeData: []string{"dv-import-controller", "dv-upload-controller", "dv-pvc-clone-controller", "dv-snapshot-clone-controller", "dv-populator-controller", "data-import-cron-controller", "storage-profile-controller"},
	}

	res24 := &KeyNode{
		name:     "OvirtVolumePopulator",
		nodeData: []string{"forklift-populator"},
	}

	res25 := &KeyNode{
		name:     "OpenstackVolumePopulator",
		nodeData: []string{"forklift-populator"},
	}

	resourceNodes = append(resourceNodes, res1, res2, res3, res4, res5, res6, res7, res8, res9, res10, res11, res12, res13, res14, res15, res16, res17, res18, res19, res20, res21, res22, res23, res24, res25)

	mapperInfo.Set("resources", resourceNodes)

	return mapperInfo
}

// represents a api group
type MapGroup struct {
	groupName      string
	groupClickable widget.Clickable
	values         []*KeyNode
	treeState      component.DiscloserState
}

func (a *MapGroup) Text() string {
	return a.groupName
}

func (a *MapGroup) GetClickable() *widget.Clickable {
	return &a.groupClickable
}

type ClickItem interface {
	Text() string
	GetClickable() *widget.Clickable
}

type MapperTab struct {
	title        string
	tabClickable widget.Clickable

	refreshButton  widget.Clickable
	refreshTooltip component.Tooltip
	refreshTipArea component.TipArea

	resList widget.List
	buttons []layout.FlexChild

	widget    layout.Widget
	allGroups []*MapGroup
	resize    component.Resize

	current      ClickItem
	detailPage   widget.Editor
	detailEditor material.EditorStyle
}

// GetClickable implements PanelTab.
func (a *MapperTab) GetClickable() *widget.Clickable {
	return &a.tabClickable
}

// GetTabButtons implements PanelTab.
func (a *MapperTab) GetTabButtons(th *material.Theme) []layout.FlexChild {
	return a.buttons
}

// GetTitle implements PanelTab.
func (a *MapperTab) GetTitle() string {
	return a.title
}

// GetWidget implements PanelTab.
func (a *MapperTab) GetWidget() layout.Widget {
	return a.widget
}

func (a *MapperTab) processClick(item ClickItem, gtx layout.Context) {
	if item.GetClickable().Clicked(gtx) {
		a.current = item
		a.detailPage.SetText(item.Text())
	}
}

func (a *MapperTab) LayoutMapGroups(th *material.Theme, gtx layout.Context, mgs *MapGroup) layout.Dimensions {

	children := make([]layout.FlexChild, 0)
	for _, v := range mgs.values {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			flatBtnText := material.Body1(th, v.name)
			flatBtnText.TextSize = unit.Sp(15)
			a.processClick(v, gtx)
			if a.current == v {
				flatBtnText.Font.Weight = font.Bold
			} else {
				flatBtnText.Font.Weight = font.Normal
			}
			return material.Clickable(gtx, v.GetClickable(), func(gtx layout.Context) layout.Dimensions {
				return layout.W.Layout(gtx, flatBtnText.Layout)
			})
		}))
	}
	return component.SimpleDiscloser(th, &mgs.treeState).Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			label := material.H6(th, mgs.groupName)
			label.TextSize = 16
			a.processClick(mgs, gtx)
			if a.current == mgs {
				label.Font.Weight = font.Bold
			} else {
				label.Font.Weight = font.Normal
			}
			return material.Clickable(gtx, &mgs.groupClickable, func(gtx layout.Context) layout.Dimensions {
				return label.Layout(gtx)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		},
	)
}

func (t *MapperTab) populateTableContents(mapInfo *om.OrderedMap[string, []*KeyNode], refresh bool) {
	if len(t.allGroups) == 0 || refresh {

		t.allGroups = make([]*MapGroup, 0)
		for pair := mapInfo.Oldest(); pair != nil; pair = pair.Next() {
			group := &MapGroup{
				groupName: pair.Key,
				values:    pair.Value,
			}

			t.allGroups = append(t.allGroups, group)
		}
	}
}

func NewMapperTab(th *material.Theme) *MapperTab {

	tab := &MapperTab{
		title:     "mapper",
		allGroups: make([]*MapGroup, 0),
		buttons:   make([]layout.FlexChild, 0),
	}

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
			logger.Info("refreshing")
		}
		return layout.Inset{Top: 4, Bottom: 0, Left: 0, Right: 4}.Layout(gtx, reloadBtn.Layout)
	})

	tab.buttons = append(tab.buttons, rigid1)

	tab.resList.Axis = layout.Vertical
	tab.resize.Ratio = 0.4

	mapperInfo := GetStaticMapper()

	tab.populateTableContents(mapperInfo, false)

	tab.detailEditor = material.Editor(th, &tab.detailPage, "details")
	tab.detailEditor.Font.Typeface = "monospace"
	//	tab.detailEditor.Font.Weight = font.Bold
	tab.detailEditor.TextSize = unit.Sp(15)
	tab.detailEditor.LineHeight = unit.Sp(15)
	if tab.current != nil {
		tab.detailPage.SetText(tab.current.Text())
	}

	tab.widget = func(gtx layout.Context) layout.Dimensions {
		return tab.resize.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				return material.List(th, &tab.resList).Layout(gtx, len(tab.allGroups),
					func(gtx layout.Context, index int) layout.Dimensions {
						agrp := tab.allGroups[index]
						return layout.UniformInset(unit.Dp(4)).Layout(gtx,
							func(gtx layout.Context) layout.Dimensions {
								return tab.LayoutMapGroups(th, gtx, agrp)
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
	return tab
}
