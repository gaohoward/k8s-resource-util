package panels

import (
	"image"
	"slices"
	"strings"

	"gaohoward.tools/k8s/resutil/pkg/common"
	"gaohoward.tools/k8s/resutil/pkg/graphics"
	"gaohoward.tools/k8s/resutil/pkg/logs"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	om "github.com/wk8/go-ordered-map/v2"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	CONTEXT_KEY_NAMESPACE         = "app.panel.in_kube.show.namespace"
	CONTEXT_KEY_API_RESOURCE      = "app.panel.in_kube.show.apiresource"
	CONTEXT_KEY_API_SEARCH_RESULT = "app.panel.in_kube.api.search.result"
)

type InKubeTab struct {
	title        string
	tabClickable widget.Clickable

	client   *common.K8sClient
	inLogger *zap.Logger

	buttons []layout.FlexChild

	refreshButton  widget.Clickable
	refreshTooltip component.Tooltip
	refreshTipArea component.TipArea

	exQueryButton widget.Clickable

	showApiResButton    widget.Bool
	showNamespaceButton widget.Bool

	resize1 component.Resize
	resize2 component.Resize

	searchField widget.Editor

	nsList  widget.List
	resList widget.List

	namespaceItems []*NamespaceItem

	//need order and also need search
	resourceItems   []*ResourceItem
	resourceItemMap map[string]*ResourceItem

	currentCriteria SearchCriteria
	searchResults   *om.OrderedMap[string, []*unstructured.UnstructuredList]

	grid         component.GridState
	resultResize component.Resize

	currentResultItem *SearchResultItem

	detailList layout.List
	//	detailEditor material.EditorStyle
	detailPanel *DetailPanel

	widget layout.Widget
}

type DetailPanel struct {
	detailContent layout.Widget
	owner         common.IResourceDetail
}

func (t *InKubeTab) Query() ([]*unstructured.UnstructuredList, error) {
	results := make([]*unstructured.UnstructuredList, 0)
	targetNs := t.currentCriteria.GetTargetNamespaces()
	if len(targetNs) == 0 {
		targetNs = []string{""}
	}
	for _, ns := range targetNs {
		for pair := t.currentCriteria.Res.Oldest(); pair != nil; pair = pair.Next() {
			targetNs := ns
			g, v, r := pair.Value.Gvr()
			if !pair.Value.res.Namespaced {
				targetNs = ""
			}
			result, err := t.client.FetchGVRInstances(g, v, r, targetNs)
			if err != nil {
				logger.Warn("failed to query the cluster", zap.Error(err))
				t.inLogger.Info("failed to query resource", zap.String("name", r), zap.String("err", err.Error()))
				continue
			}
			results = append(results, result)
		}
	}
	t.searchResults.Set(t.currentCriteria.Compile(), results)
	return results, nil

}

type SearchCriteria struct {
	Ns         *om.OrderedMap[string, string]
	Res        *om.OrderedMap[string, *ResourceItem]
	Valid      bool
	InvalidMsg string
	changed    bool
}

func (s SearchCriteria) GetTargetNamespaces() []string {
	allNs := make([]string, 0)
	for pair := s.Ns.Oldest(); pair != nil; pair = pair.Next() {
		allNs = append(allNs, pair.Value)
	}
	return allNs
}

// add or update
func (s *SearchCriteria) AddNs(ns string) {
	s.Ns.Set(ns, ns)
	s.changed = true
}

func (s *SearchCriteria) RemoveNs(ns string) {
	s.Ns.Delete(ns)
	s.changed = true
}

func (s *SearchCriteria) AddRes(gvr string, item *ResourceItem) {
	s.Res.Set(gvr, item)
	s.changed = true
}

func (s *SearchCriteria) RemoveRes(gvr string) {
	s.Res.Delete(gvr)
	s.changed = true
}

func (s *SearchCriteria) Reset() {
	s.Ns = om.New[string, string]()
	s.Res = om.New[string, *ResourceItem]()
	s.InvalidMsg = ""
	s.Valid = true
	s.changed = false
}

func (s *SearchCriteria) Changed() bool {
	return s.changed
}

func (s *SearchCriteria) Compile() string {

	builder := strings.Builder{}
	builder.WriteString("ns=")
	for pair := s.Ns.Oldest(); pair != nil; pair = pair.Next() {
		builder.WriteString(pair.Value)
		builder.WriteString(" ")
	}

	builder.WriteString(", res=")
	for pair := s.Res.Oldest(); pair != nil; pair = pair.Next() {
		builder.WriteString(pair.Key)
		builder.WriteString(" ")
	}

	s.changed = false

	return strings.TrimSpace(builder.String())
}

func (s *SearchCriteria) ParseItems(kind string, expr string, t *InKubeTab) {

	kind = strings.TrimSpace(kind)

	if kind != "ns" && kind != "res" {
		s.Valid = false
		s.InvalidMsg = "Invalid criteria, format: [ns|res]=item1 item2 ..."
		return
	}

	allItems := strings.TrimSpace(expr)
	iParts := strings.Split(allItems, " ")
	for _, item := range iParts {
		item = strings.TrimSpace(item)
		if item != "" {
			if kind == "ns" {
				s.Ns.Set(item, item)
			} else {
				if entry, ok := t.resourceItemMap[item]; ok {
					s.Res.Set(item, entry)
				}
			}
		}
	}
}

// ns=ns1 ns2 ns3, res=v1/pods v2/res
// also valid: ns=ns1 ns2, ns=ns3 ns4, res=v1/pods, res=v1/pods...
func (t *InKubeTab) ParseSearchString(searchText string) *SearchCriteria {
	t.currentCriteria.Reset()

	parts := strings.Split(searchText, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		exParts := strings.Split(p, "=")
		if len(exParts) != 2 {
			t.currentCriteria.Valid = false
			t.currentCriteria.InvalidMsg = "Invalid criteria, format: [ns|res]=item1 item2 ..."
			return &t.currentCriteria
		}

		t.currentCriteria.ParseItems(exParts[0], exParts[1], t)
	}

	return &t.currentCriteria
}

type NamespaceItem struct {
	ns       string
	nsBtn    widget.Clickable
	selected bool
}

type ResourceItem struct {
	verInfo  *common.ApiVerGvName
	res      *v1.APIResource
	resBtn   widget.Clickable
	selected bool
}

func (r *ResourceItem) Gvr() (string, string, string) {
	gv := strings.Split(r.verInfo.Gv, "/")
	if len(gv) == 2 {
		return gv[0], gv[1], r.verInfo.Name
	}
	return "", gv[0], r.verInfo.Name
}

func (r *ResourceItem) GetGvr() string {
	return r.verInfo.Gv + "/" + r.verInfo.Name
}

func (t *InKubeTab) RefreshNamespaces() {
	t.namespaceItems = make([]*NamespaceItem, 0)
	allNamespaces, err := t.client.FetchAllNamespaces()
	if err != nil {
		logger.Info("failed to refresh namespaces")
		return
	}
	// sort the namespaces
	slices.SortFunc(allNamespaces, func(a, b string) int {
		return strings.Compare(a, b)
	})
	for _, n := range allNamespaces {
		t.namespaceItems = append(t.namespaceItems, &NamespaceItem{
			ns: n,
		})
	}

}

func (t *InKubeTab) RefreshApiResources(force bool) {
	t.resourceItems = make([]*ResourceItem, 0)
	t.resourceItemMap = make(map[string]*ResourceItem, 0)

	allResInfo := t.client.FetchAllApiResources(force)
	if allResInfo != nil {
		for _, n := range allResInfo.ResList {
			for _, r := range n.APIResources {
				item := &ResourceItem{
					verInfo: common.NewApiVerGvName("", n.GroupVersion, r.Name),
					res:     &r,
				}
				t.resourceItems = append(t.resourceItems, item)
				t.resourceItemMap[n.GroupVersion+"/"+r.Name] = item
			}
		}
		slices.SortFunc(t.resourceItems, func(a, b *ResourceItem) int {
			return strings.Compare(a.GetGvr(), b.GetGvr())
		})
	}
}

type SearchResultItem struct {
	item          *unstructured.Unstructured
	clickable     widget.Clickable
	label0        material.LabelStyle
	details       []common.IResourceDetail
	currentDetail common.IResourceDetail
	statusInfo    common.ResStatusInfo
}

func (s *SearchResultItem) SupportStatus() bool {
	return s.item.GetKind() == "Pod"
}

func (s *SearchResultItem) GetStatusIcon(th *material.Theme) common.ResStatusInfo {
	if s.item.GetKind() == "Pod" && s.statusInfo == nil {

		var pod corev1.Pod

		err := runtime.DefaultUnstructuredConverter.
			FromUnstructured(s.item.Object, &pod)
		if err != nil {
			logger.Error("error convert pos", zap.Error(err))
		} else {

			podStatusInfo := common.NewPodStatusInfo(pod.GetName(), th)

			for _, con := range pod.Status.InitContainerStatuses {
				if con.State.Terminated != nil {
					if con.State.Terminated.ExitCode != 0 {
						podStatusInfo.SetContainerStatus(con.Name, common.ContainerTerminatedWithError, con.State.Terminated.Reason, th)
						podStatusInfo.SetStatus(common.PodError, "container terminated with error", th)
					} else {
						podStatusInfo.SetContainerStatus(con.Name, common.ContainerTerminated, con.State.Terminated.Message, th)
						podStatusInfo.SetStatus(common.ContainerTerminated, "container terminated normally", th)
					}
				} else if con.State.Waiting != nil {
					podStatusInfo.SetContainerStatus(con.Name, common.ContainerError, con.State.Waiting.Reason, th)
					podStatusInfo.SetStatus(common.PodError, "container waiting", th)
				} else if con.State.Running != nil {
					podStatusInfo.SetContainerStatus(con.Name, common.ContainerRunning, con.State.Running.StartedAt.Time.String(), th)
					podStatusInfo.SetStatus(common.PodRunning, "container running", th)
				} else {
					podStatusInfo.SetContainerStatus(con.Name, common.ContainerUnknown, "unknown state", th)
					podStatusInfo.SetStatus(common.PodUnknown, "container state unknown", th)
				}
			}

			allContainerRunning := true

			for _, con := range pod.Status.ContainerStatuses {
				if con.State.Terminated != nil {
					if con.State.Terminated.ExitCode != 0 {
						podStatusInfo.SetContainerStatus(con.Name, common.ContainerTerminatedWithError, con.State.Terminated.Reason, th)
						podStatusInfo.SetStatus(common.PodError, "container terminated with error", th)
						allContainerRunning = false
					} else {
						podStatusInfo.SetContainerStatus(con.Name, common.ContainerTerminated, con.State.Terminated.Message, th)
						podStatusInfo.SetStatus(common.ContainerTerminated, "container terminated normally", th)
						allContainerRunning = false
					}
				} else if con.State.Waiting != nil {
					podStatusInfo.SetContainerStatus(con.Name, common.ContainerError, con.State.Waiting.Reason, th)
					podStatusInfo.SetStatus(common.PodError, "container waiting", th)
					allContainerRunning = false
				} else if con.State.Running != nil {
					podStatusInfo.SetContainerStatus(con.Name, common.ContainerRunning, con.State.Running.StartedAt.Time.String(), th)
				} else {
					podStatusInfo.SetContainerStatus(con.Name, common.ContainerUnknown, "unknown state", th)
					podStatusInfo.SetStatus(common.PodUnknown, "container state unknown", th)
					allContainerRunning = false
				}
			}
			if allContainerRunning {
				podStatusInfo.SetStatus(common.PodRunning, "container running", th)
			}
			s.statusInfo = podStatusInfo
			return podStatusInfo
		}
	}
	return s.statusInfo
}

func (s *SearchResultItem) GetDetails(gtx layout.Context, th *material.Theme) []common.IResourceDetail {
	if len(s.details) == 0 {
		s.details = make([]common.IResourceDetail, 0)
		s.details = append(s.details, NewYamlDetail(th, s.item))
		extDetails := GetExtApiDetails(s.item, th, s.statusInfo)
		if len(extDetails) > 0 {
			s.details = append(s.details, extDetails...)
		}
	}
	return s.details
}

func getSearchResultList(th *material.Theme, result []*unstructured.UnstructuredList) []*SearchResultItem {
	resultList := make([]*SearchResultItem, 0)
	itemList := common.GetAllUnstructuredItems(result)
	for _, item := range itemList {
		resultList = append(resultList, &SearchResultItem{
			item:   item,
			label0: material.Label(th, unit.Sp(15), ""),
		})
	}
	return resultList
}

// e.g. mypod, Pod, default
// namespace should be empty if resource is not namespaced
var headings = []string{"Name", "Kind", "Namespace"}

func (tab *InKubeTab) layoutCurrentDetail(th *material.Theme, gtx layout.Context) layout.Dimensions {
	title := tab.currentResultItem.item.GetKind() + ": " + tab.currentResultItem.item.GetName()
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if tab.currentResultItem.SupportStatus() {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						sicon := tab.currentResultItem.GetStatusIcon(th)
						if sicon != nil {
							return sicon.Layout(gtx, 20, nil)
						}
						logger.Info("No status icon for resource", zap.String("name", tab.currentResultItem.item.GetName()))
						return layout.Dimensions{}
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Label(th, unit.Sp(20), title).Layout(gtx)
					}),
				)
			}
			return material.Label(th, unit.Sp(20), title).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			div := component.Divider(th)
			div.Inset.Top = unit.Dp(0)
			return div.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			resDetails := tab.currentResultItem.GetDetails(gtx, th)
			return tab.detailList.Layout(gtx, len(resDetails), func(gtx layout.Context, index int) layout.Dimensions {
				detail := resDetails[index]
				if detail.GetClickable().Clicked(gtx) {
					if tab.currentResultItem.currentDetail != detail {
						if tab.currentResultItem.currentDetail != nil {
							tab.currentResultItem.currentDetail.SetSelected(false)
						}
						tab.currentResultItem.currentDetail = detail
						detail.SetSelected(true)
					}
				}

				return layout.Inset{Top: 0, Bottom: 0, Left: unit.Dp(6), Right: unit.Dp(4)}.Layout(gtx,
					func(gtx layout.Context) layout.Dimensions {
						return material.Clickable(gtx, detail.GetClickable(), detail.GetLabel())
					})
			})
		}),
		layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(12), Bottom: 0, Left: unit.Dp(4), Right: unit.Dp(0)}.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					if tab.currentResultItem.currentDetail != nil {
						return layout.Inset{Top: 0, Bottom: 0, Left: unit.Dp(4), Right: 0}.Layout(gtx, tab.currentResultItem.currentDetail.GetContent())
					}
					return layout.Dimensions{}
				},
			)
		}),
	)
}

func NewInKubeTab(th *material.Theme, client *common.K8sClient) *InKubeTab {

	common.RegisterContext(CONTEXT_KEY_NAMESPACE, false, true)
	common.RegisterContext(CONTEXT_KEY_API_RESOURCE, false, true)
	common.RegisterContext(CONTEXT_KEY_API_SEARCH_RESULT, nil, true)

	tab := &InKubeTab{
		title:    "in-kube",
		client:   client,
		inLogger: logs.GetLogger(logs.IN_APP_LOGGER_NAME),
		detailPanel: &DetailPanel{
			owner: nil,
		},
	}

	tab.currentCriteria = SearchCriteria{
		Ns:    om.New[string, string](),
		Res:   om.New[string, *ResourceItem](),
		Valid: true,
	}

	tab.searchResults = om.New[string, []*unstructured.UnstructuredList]()

	tab.RefreshNamespaces()
	tab.RefreshApiResources(false)

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
			tab.RefreshNamespaces()
			tab.RefreshApiResources(true)
			tab.searchField.SetText("")
			tab.currentCriteria.Reset()
		}
		return layout.Inset{Top: 4, Bottom: 0, Left: 0, Right: 4}.Layout(gtx, reloadBtn.Layout)
	})

	tab.buttons = append(tab.buttons, rigid1)

	tab.resize1.Ratio = 0.2
	tab.resize2.Ratio = 0.3
	tab.resultResize.Ratio = 0.4

	tab.nsList.Axis = layout.Vertical
	tab.resList.Axis = layout.Vertical

	tab.searchField.SingleLine = true
	tab.searchField.LineHeight = unit.Sp(18)
	tab.searchField.LineHeightScale = 0.8
	tab.searchField.ReadOnly = true

	exBtn := material.IconButton(th, &tab.exQueryButton, graphics.ExecIcon, "go")
	exBtn.Inset = layout.Inset{Top: 0, Bottom: 0, Left: 1, Right: 1}
	exBtn.Size = unit.Dp(22)

	nsSwitch := material.CheckBox(th, &tab.showNamespaceButton, "ns")
	nsSwitch.Size = unit.Dp(16)
	nsSwitch.CheckBox.Value = true
	common.SetContextBool(CONTEXT_KEY_NAMESPACE, tab.showNamespaceButton.Value, nil)

	resSwitch := material.CheckBox(th, &tab.showApiResButton, "api")
	resSwitch.Size = unit.Dp(16)
	resSwitch.CheckBox.Value = true
	common.SetContextBool(CONTEXT_KEY_API_RESOURCE, tab.showApiResButton.Value, nil)

	searchBar := func(gtx layout.Context) layout.Dimensions {
		//only set text when changes!
		if tab.currentCriteria.Changed() {
			tab.searchField.SetText(tab.currentCriteria.Compile())
		}
		editor := material.Editor(th, &tab.searchField, "search criteria")
		editor.Font.Weight = font.Bold
		editor.Color = common.COLOR.Blue()
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return graphics.SearchIcon.Layout(gtx, common.COLOR.Blue())
			}),
			layout.Flexed(1.0, editor.Layout),
		)
	}

	nsPanel := func(gtx layout.Context) layout.Dimensions {
		if len(tab.namespaceItems) == 0 {
			return material.H6(th, "no namespaces availale").Layout(gtx)
		}
		return material.List(th, &tab.nsList).Layout(gtx, len(tab.namespaceItems), func(gtx layout.Context, index int) layout.Dimensions {
			item := tab.namespaceItems[index]
			if item.nsBtn.Clicked(gtx) {
				item.selected = !item.selected
				if item.selected {
					tab.currentCriteria.AddNs(item.ns)
				} else {
					tab.currentCriteria.RemoveNs(item.ns)
				}
			}
			selectionMarker := " "
			if item.selected {
				selectionMarker = "*"
			}
			return material.Clickable(gtx, &item.nsBtn, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(layout.Spacer{Width: 4}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						marker := material.Label(th, unit.Sp(16), selectionMarker)
						gtx.Constraints.Min.X = 16
						return marker.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						flatBtnText := material.Label(th, unit.Sp(16), item.ns)
						if item.selected {
							flatBtnText.Font.Weight = font.Bold
						}
						return flatBtnText.Layout(gtx)
					}),
				)
			})
		})
	}

	apiResPanel := func(gtx layout.Context) layout.Dimensions {
		if len(tab.resourceItems) == 0 {
			return material.H6(th, "no api resources availale").Layout(gtx)
		}
		return material.List(th, &tab.resList).Layout(gtx, len(tab.resourceItems), func(gtx layout.Context, index int) layout.Dimensions {
			item := tab.resourceItems[index]
			if item.resBtn.Clicked(gtx) {
				item.selected = !item.selected
				if item.selected {
					tab.currentCriteria.AddRes(item.GetGvr(), item)
				} else {
					tab.currentCriteria.RemoveRes(item.GetGvr())
				}
			}
			selectionMarker := " "
			if item.selected {
				selectionMarker = "*"
			}
			return material.Clickable(gtx, &item.resBtn, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(layout.Spacer{Width: 4}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						marker := material.Label(th, unit.Sp(16), selectionMarker)
						gtx.Constraints.Min.X = 16
						return marker.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						flatBtnText := material.Label(th, unit.Sp(16), item.verInfo.Gv+"/"+item.verInfo.Name)
						if item.selected {
							flatBtnText.Font.Weight = font.Bold
						}
						return flatBtnText.Layout(gtx)
					}),
				)
			})
		})
	}

	resultPanel := func(gtx layout.Context) layout.Dimensions {
		result, _ := common.GetContextData(CONTEXT_KEY_API_SEARCH_RESULT)
		if result == nil {
			return material.H5(th, "No result.").Layout(gtx)
		}
		if uList, ok := result.([]*SearchResultItem); ok {

			// Configure a label styled to be a heading.
			headingLabel := material.Body1(th, "")
			headingLabel.Font.Weight = font.Bold
			headingLabel.TextSize = unit.Sp(14)
			headingLabel.Alignment = text.Start
			headingLabel.MaxLines = 1

			// Measure the height of a heading row.
			inset := layout.UniformInset(unit.Dp(2))
			orig := gtx.Constraints
			gtx.Constraints.Min = image.Point{}
			dims := inset.Layout(gtx, headingLabel.Layout)
			gtx.Constraints = orig

			//populate result
			return tab.resultResize.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return component.Table(th, &tab.grid).Layout(gtx, len(uList), len(headings),
						func(axis layout.Axis, index, constraint int) int {
							unit := int(constraint / 10)
							switch axis {
							case layout.Horizontal:
								switch index {
								case 0:
									//give name a bit extra space
									return int(unit * 5)
								case 1:
									return int(unit * 2)
								case 2:
									return int(unit * 3)
								default:
									return 0
								}
							default:
								return dims.Size.Y + 2
							}
						},
						func(gtx layout.Context, index int) layout.Dimensions {
							return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								headingLabel.Text = headings[index]
								return headingLabel.Layout(gtx)
							})
						},
						func(gtx layout.Context, row, col int) layout.Dimensions {
							rowItem := uList[row]
							value := ""
							switch col {
							case 0:
								value = rowItem.item.GetName()
							case 1:
								value = rowItem.item.GetKind()
							case 2:
								value = rowItem.item.GetNamespace()
							}

							if col == 0 {
								rowItem.label0.Text = value
								if rowItem.clickable.Clicked(gtx) {
									if tab.currentResultItem != rowItem {
										tab.currentResultItem = rowItem
									}
								}
								if tab.currentResultItem == rowItem {
									rowItem.label0.Font.Weight = font.Bold
								} else {
									rowItem.label0.Font.Weight = font.Normal
								}

								if statusIcon := rowItem.GetStatusIcon(th); statusIcon != nil {
									newIcon := common.NewStatusIcon(statusIcon.GetStatus(), statusIcon.GetReason(), th)

									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return newIcon.Layout(gtx, unit.Dp(14), &layout.Inset{Top: 3, Bottom: 0, Left: 1, Right: 2})
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return material.Clickable(gtx, &rowItem.clickable, func(gtx layout.Context) layout.Dimensions {
												return rowItem.label0.Layout(gtx)
											})
										}),
									)
								}
								return material.Clickable(gtx, &rowItem.clickable, func(gtx layout.Context) layout.Dimensions {
									return rowItem.label0.Layout(gtx)
								})
							}
							l := material.Label(th, unit.Sp(15), value)
							return l.Layout(gtx)
						},
					)
				},
				func(gtx layout.Context) layout.Dimensions {
					//details
					if tab.currentResultItem != nil {
						return tab.layoutCurrentDetail(th, gtx)
					}
					return layout.Dimensions{}
				},
				common.VerticalSplitHandler,
			)
		}
		//invalid data from context
		return material.H5(th, "Invalid Context Data!").Layout(gtx)
	}

	tab.widget = func(gtx layout.Context) layout.Dimensions {

		if tab.showApiResButton.Pressed() {
			if tab.showApiResButton.Update(gtx) {
				common.SetContextBool(CONTEXT_KEY_API_RESOURCE, tab.showApiResButton.Value, nil)
			}
		}

		if tab.showNamespaceButton.Pressed() {
			if tab.showNamespaceButton.Update(gtx) {
				common.SetContextBool(CONTEXT_KEY_NAMESPACE, tab.showNamespaceButton.Value, nil)
			}
		}

		showRes, _, err := common.GetContextBool(CONTEXT_KEY_API_RESOURCE)
		if err != nil {
			logger.Info("Failed to get value from context", zap.Error(err))
			showRes = false
		}
		showNs, _, err := common.GetContextBool(CONTEXT_KEY_NAMESPACE)
		if err != nil {
			logger.Info("Failed to get value from context", zap.Error(err))
			showRes = false
		}

		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(10), Bottom: 0, Left: 0, Right: 0}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					// search bar
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
						//search selection (res and ns)
						layout.Rigid(nsSwitch.Layout),
						layout.Rigid(resSwitch.Layout),
						layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Top: 0, Bottom: 0, Left: unit.Dp(10), Right: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Vertical, Alignment: layout.End}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return layout.Inset{Top: 0, Bottom: 0, Left: 0, Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return searchBar(gtx)
										})
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										div := component.Divider(th)
										div.Inset = layout.Inset{Top: 0, Bottom: 0, Left: 0, Right: 0}
										div.Thickness = unit.Dp(2)
										return div.Layout(gtx)
									}),
								)
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if tab.exQueryButton.Clicked(gtx) {
								result, _ := tab.Query()
								logger.Info("result queried", zap.Int("size", len(result)))
								resultList := getSearchResultList(th, result)
								common.SetContextData(CONTEXT_KEY_API_SEARCH_RESULT, resultList, nil)
							}

							return layout.Inset{Top: 0, Bottom: 0, Left: 0, Right: unit.Dp(8)}.Layout(gtx,
								func(gtx layout.Context) layout.Dimensions {
									return exBtn.Layout(gtx)
								},
							)
						}),
					)
				})
			}),
			layout.Rigid(layout.Spacer{Height: 10}.Layout),
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				// main area
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
						if showNs && showRes {
							return tab.resize1.Layout(gtx,
								nsPanel,
								func(gtx layout.Context) layout.Dimensions {
									return tab.resize2.Layout(gtx,
										apiResPanel,
										resultPanel,
										common.VerticalSplitHandler,
									)
								},
								common.VerticalSplitHandler,
							)
						}
						if showNs {
							return tab.resize1.Layout(gtx,
								nsPanel,
								resultPanel,
								common.VerticalSplitHandler,
							)
						}
						if showRes {
							return tab.resize1.Layout(gtx,
								apiResPanel,
								resultPanel,
								common.VerticalSplitHandler,
							)
						}
						return resultPanel(gtx)
					}),
				)
			}),
		)
	}

	return tab
}

func (a *InKubeTab) GetClickable() *widget.Clickable {
	return &a.tabClickable
}

// GetTabButtons implements PanelTab.
func (a *InKubeTab) GetTabButtons(th *material.Theme) []layout.FlexChild {
	return a.buttons
}

func (a *InKubeTab) GetTitle() string {
	return a.title
}

// GetWidget implements PanelTab.
func (a *InKubeTab) GetWidget() layout.Widget {
	return a.widget
}
