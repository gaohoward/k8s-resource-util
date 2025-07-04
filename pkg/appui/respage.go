package appui

import (
	"fmt"
	"image"
	"image/color"

	"slices"

	"gaohoward.tools/k8s/resutil/pkg/common"
	"gaohoward.tools/k8s/resutil/pkg/graphics"
	"gaohoward.tools/k8s/resutil/pkg/k8sservice"
	"gaohoward.tools/k8s/resutil/pkg/logs"
	"gaohoward.tools/k8s/resutil/pkg/panels"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
)

var NO_SCHEMA = "No CRD for collections"

type ActiveResource struct {
	Instance           common.Resource
	btn                widget.Clickable
	closeBtn           widget.Clickable
	menuState          component.MenuState
	contextArea        component.ContextArea
	MenuBtnCloseOthers widget.Clickable
	MenuBtnCloseAll    widget.Clickable
	MenuBtnCloseLeft   widget.Clickable
	MenuBtnCloseRight  widget.Clickable
}

func (a *ActiveResource) UpdateFromRepo(node common.INode) {
	a.Instance.Update(node)
}

type ActiveResourceSet struct {
	resList    []*ActiveResource
	resMap     map[string]*ActiveResource
	closeQueue []string
}

func (a *ActiveResourceSet) GetLast() *ActiveResource {
	return a.Get(len(a.resList) - 1)
}

func (a *ActiveResourceSet) pushClose(rcs ...*ActiveResource) {
	for _, rc := range rcs {
		a.closeQueue = append(a.closeQueue, rc.Instance.GetId())
	}
}

func (a *ActiveResourceSet) prune() string {
	lastIndex := -1
	for _, id := range a.closeQueue {
		lastIndex = a.Remove(id)
	}
	a.closeQueue = a.closeQueue[:0]

	if a.Size() > 0 {
		if lastIndex != -1 {
			if a.Size() > lastIndex {
				return a.Get(lastIndex).Instance.GetId()
			} else {
				lastIndex--
				if lastIndex < 0 {
					return ""
				}
				return a.Get(lastIndex).Instance.GetId()
			}
		}
	}
	return ""
}

func (a *ActiveResourceSet) Remove(resId string) int {
	delete(a.resMap, resId)
	for i, res := range a.resList {
		if res.Instance.GetId() == resId {
			a.resList = slices.Delete(a.resList, i, i+1)
			return i
		}
	}
	return -1
}

func (a *ActiveResourceSet) Get(index int) *ActiveResource {
	if index < 0 || index >= len(a.resList) {
		panic("index out of scope")
	}
	return a.resList[index]
}

func (a *ActiveResourceSet) Size() int {
	return len(a.resList)
}

func (a *ActiveResourceSet) Add(ac *ActiveResource) {
	a.resList = append(a.resList, ac)
	a.resMap[ac.Instance.GetId()] = ac
}

func (a *ActiveResourceSet) FindInstance(apiVer string, name string) *ActiveResource {
	for _, inst := range a.resList {
		if inst.Instance.GetSpecApiVer() == apiVer && inst.Instance.GetName() == name {
			return inst
		}
	}
	return nil
}

func (a *ActiveResourceSet) FindById(id string) *ActiveResource {
	if resource, exists := a.resMap[id]; exists {
		return resource
	}
	return nil
}

func NewActiveResourceSet() *ActiveResourceSet {
	return &ActiveResourceSet{
		resList:    make([]*ActiveResource, 0),
		resMap:     make(map[string]*ActiveResource),
		closeQueue: make([]string, 0),
	}
}

type ResourcePage struct {
	resourceManager  common.ResourceManager
	current          common.Resource
	activeResources  *ActiveResourceSet
	primarySplit     component.Resize
	crPanel          widget.Editor
	bothPanel        component.Resize
	crdEditor        *common.ReadOnlyEditor
	loadErr          error
	ch               chan string
	refreshCh        chan int
	crEditor         material.EditorStyle
	fullEditor       material.EditorStyle
	editorBtnDeploy  widget.Clickable
	DeployBtnTooltip component.Tooltip
	DeployBtnTipArea component.TipArea

	editorBtnSave  widget.Clickable
	SaveBtnTooltip component.Tooltip
	SaveBtnTipArea component.TipArea

	k8sClient         k8sservice.K8sService
	appPanel          *panels.AppPanel
	deployedResources *k8sservice.DeployedResources
	tabsList          layout.List
	showSchema        widget.Bool
}

func (rp ResourcePage) GetPanel() *panels.AppPanel {
	return rp.appPanel
}

func (rp *ResourcePage) SetResourceManager(rc common.ResourceManager) {
	rp.resourceManager = rc
}

func (rp *ResourcePage) RemoveRefs(resId string, holder map[string]common.INode) {
	rp.activeResources.Remove(resId)
	if n, ok := holder[resId]; ok {
		if col, ok := n.(*common.Collection); ok {
			for _, res := range col.GetAllResources() {
				rp.activeResources.Remove(res)
			}
		}
	}
	if rp.current != nil && rp.current.GetId() == resId {
		rp.current = nil
	}
}

// when the resource collections changes (such as reload)
// call this message to update the contents in open tabs
func (rp *ResourcePage) UpdateActiveContents(resIds []string, updateHolder map[string]common.INode) {

	for _, id := range resIds {
		res := rp.activeResources.FindById(id)
		if res != nil {
			if node, ok := updateHolder[id]; ok {
				res.UpdateFromRepo(node)
			} else {
				logger.Warn("Resource not found", zap.String("id", id))
			}
		}
	}

	if rp.current != nil {
		for _, id := range resIds {
			if id == rp.current.GetId() {
				if n, ok := updateHolder[id]; ok {
					if rs, ok1 := n.(*common.ResourceNode); ok1 {
						newCr := rs.GetResource().GetCR()
						rp.crPanel.SetText(newCr)
					} else if c, ok1 := n.(*common.Collection); ok1 {
						rp.crPanel.SetText(c.GetConfigContent())
					}
				}
			}
		}
	}
}

func (rp *ResourcePage) Init(rtclient k8sservice.K8sService, refreshCh chan int, th *material.Theme) {
	rp.current = nil
	rp.activeResources = NewActiveResourceSet()

	rp.primarySplit = component.Resize{Ratio: 0.4}
	rp.bothPanel = component.Resize{Ratio: 0.5}
	rp.ch = make(chan string, 1)
	rp.refreshCh = refreshCh

	rp.crEditor = material.Editor(th, &rp.crPanel, "")
	rp.crEditor.Font.Typeface = "monospace"
	rp.crEditor.Font.Weight = font.Bold
	rp.crEditor.TextSize = unit.Sp(16)
	rp.crEditor.LineHeight = unit.Sp(16)

	rp.crdEditor = common.NewReadOnlyEditor(th, "Schema", 16, nil)

	rp.SaveBtnTooltip = component.DesktopTooltip(th, "Save")
	rp.DeployBtnTooltip = component.DesktopTooltip(th, "Deploy")

	rp.k8sClient = rtclient

	rp.deployedResources = k8sservice.NewDeployedResources()

	rp.appPanel = panels.GetAppPanel(th, rp.deployedResources, rtclient, rp.resourceManager)
}

func (rp *ResourcePage) WaitForLoading(rc common.Resource) {
	<-rp.ch
	rp.refreshCh <- 1
}

func (rp *ResourcePage) isCurrent(ac *ActiveResource) bool {
	if rp.current != nil {
		return rp.current.GetId() == ac.Instance.GetId()
	}
	return false
}

func (rp *ResourcePage) loadResource(rc common.Resource) {

	if rc.IsSpecLoaded() {
		return
	}

	go func(ch chan string) {

		var crd string
		var err error

		if _, ok := rc.(*common.ResourceInstance); ok {
			apiVer := rc.GetSpecApiVer()
			allRes := k8sservice.GetK8sService().FetchAllApiResources(false)
			schemaFound := false
			if allRes != nil {
				entry := allRes.FindApiResource(apiVer)
				if entry != nil && entry.Schema != "" {
					crd = entry.Schema
					schemaFound = true
				}
			}
			if !schemaFound {
				if spec, ok := common.BuiltinResSpecMap[apiVer]; ok {
					crd = spec.Schema
					schemaFound = true
				} else {
					crd = "schema not available"
				}
			}
		} else {
			//ResourceCollection
			crd = rc.GetSpecSchema()
			err = nil
		}
		if err != nil {
			rp.loadErr = err
			ch <- err.Error()
		} else {
			rc.SetSpecSchema(crd)
			rc.SetSpecLoaded(true)
			rp.loadErr = nil
			ch <- "ok"
		}
	}(rp.ch)

	go rp.WaitForLoading(rc)
}

func (rp *ResourcePage) UpdateCRD() {
	if rp.current != nil {
		if cur, ok := rp.current.(*common.ResourceInstance); ok {
			cont := cur.Spec.GetSchema()
			if rp.loadErr != nil {
				cont += "\n error: " + rp.loadErr.Error()
			}
			rp.crdEditor.SetText(&cont)
		} else {
			rp.crdEditor.SetText(&NO_SCHEMA)
		}
	}
}

func (rp *ResourcePage) setCurrent(newCurrent *ActiveResource) {
	if !rp.isCurrent(newCurrent) {
		rp.loadResource(newCurrent.Instance)

		rp.current = newCurrent.Instance
		rp.crPanel.SetText(rp.current.GetCR())
		schema := rp.current.GetSpecSchema()
		rp.crdEditor.SetText(&schema)
	}
}

func (rp *ResourcePage) Activate(id string) {
	inst := rp.FindInstanceById(id)

	if inst == nil {
		//shouldn't happen
		return
	}
	rp.setCurrent(inst)
}

// Open a new resource with template content
func (rp *ResourcePage) OpenTemplate(kind common.BuiltinKind) {
	apiVer := kind.ToApiVer()
	inst := rp.FindInstance(apiVer, "")
	if inst == nil {
		resInst := common.NewBuiltinInstance(apiVer, 0)
		inst = rp.AddActiveResource(resInst, true)
	}
	rp.Activate(inst.Instance.GetId())
}

func (rp *ResourcePage) FindInstanceById(id string) *ActiveResource {
	return rp.activeResources.FindById(id)
}

func (rp *ResourcePage) FindInstance(apiVer string, name string) *ActiveResource {
	return rp.activeResources.FindInstance(apiVer, name)
}

func (rp *ResourcePage) AddActiveResource(rc common.Resource, activate bool) *ActiveResource {
	ac := rp.FindInstanceById(rc.GetId())
	if ac == nil {
		ac = &ActiveResource{
			Instance: rc,
		}
		rp.activeResources.Add(ac)
	}
	if activate {
		rp.Activate(rc.GetId())
	}
	return ac
}

func (rp *ResourcePage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {

	if rp.loadErr != nil {
		err := rp.loadErr.Error()
		rp.crdEditor.SetHint(&err)
	}

	var crEditorTab layout.Widget = func(gtx layout.Context) layout.Dimensions {
		if rp.activeResources.Size() == 0 {
			return layout.Dimensions{}
		}

		nextCurrentId := rp.activeResources.prune()

		if nextCurrentId != "" {
			rp.Activate(nextCurrentId)
		}

		if rp.current == nil {
			//elect one last
			if rp.activeResources.Size() > 0 {
				elected := rp.activeResources.GetLast().Instance.GetId()
				rp.Activate(elected)
			}
		} else if rp.activeResources.Size() == 0 {
			rp.current = nil
		}

		return rp.tabsList.Layout(gtx, rp.activeResources.Size(), func(gtx layout.Context, index int) layout.Dimensions {
			rc := rp.activeResources.Get(index)
			clicked := rc.btn.Clicked(gtx)
			if clicked {
				rp.Activate(rc.Instance.GetId())
			}

			if rc.closeBtn.Clicked(gtx) {
				rp.Close(rc)
			}

			if rc.MenuBtnCloseOthers.Clicked(gtx) {
				rp.Close(rp.activeResources.resList[:index]...)
				rp.Close(rp.activeResources.resList[index+1:]...)
			}
			if rc.MenuBtnCloseAll.Clicked(gtx) {
				rp.Close(rp.activeResources.resList...)
			}
			if rc.MenuBtnCloseRight.Clicked(gtx) {
				rp.Close(rp.activeResources.resList[index+1:]...)
			}
			if rc.MenuBtnCloseLeft.Clicked(gtx) {
				rp.Close(rp.activeResources.resList[:index]...)
			}

			stackChildren := make([]layout.StackChild, 0)
			tabStackChild := layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{
					Top:    unit.Dp(0),
					Bottom: unit.Dp(0),
					Left:   unit.Dp(2),
					Right:  unit.Dp(0),
				}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					label := material.H6(th, rc.Instance.GetLabel())
					label.TextSize = unit.Sp(16)
					if rp.isCurrent(rc) {
						label.Font.Weight = font.Bold
					}

					button := material.Button(th, &rc.closeBtn, "x")
					button.Inset = layout.Inset{Top: 0, Bottom: 2, Left: 2, Right: 2}
					button.Background = th.Bg
					button.Color = th.Fg
					button.Font.Weight = font.Bold
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(label.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Max.Y = 64
							gtx.Constraints.Min.X = 24
							return layout.Inset{Top: 0, Bottom: 2, Left: 0, Right: 0}.Layout(gtx,
								button.Layout)
						}),
					)
				})
			})
			stackChildren = append(stackChildren, tabStackChild)

			nTabs := rp.activeResources.Size()
			if rp.isCurrent(rc) && nTabs > 1 {
				menuStackChild := layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					return rc.contextArea.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						menuItems := make([]func(gtx layout.Context) layout.Dimensions, 0)
						// add close all
						item0 := component.MenuItem(th, &rc.MenuBtnCloseOthers, "Close others")
						item0.Label.TextSize = unit.Sp(14)
						item0.Label.LineHeight = unit.Sp(14)
						menuItems = append(menuItems, item0.Layout)
						// add close all
						item1 := component.MenuItem(th, &rc.MenuBtnCloseAll, "Close all")
						item1.Label.TextSize = unit.Sp(14)
						item1.Label.LineHeight = unit.Sp(14)
						menuItems = append(menuItems, item1.Layout)
						if index < nTabs-1 {
							// close right
							item := component.MenuItem(th, &rc.MenuBtnCloseRight, "Close right")
							item.Label.TextSize = unit.Sp(14)
							item.Label.LineHeight = unit.Sp(14)
							menuItems = append(menuItems, item.Layout)
						}
						if index > 0 {
							item := component.MenuItem(th, &rc.MenuBtnCloseLeft, "Close left")
							item.Label.TextSize = unit.Sp(14)
							item.Label.LineHeight = unit.Sp(14)
							menuItems = append(menuItems, item.Layout)
						}
						rc.menuState = component.MenuState{
							Options: menuItems,
						}
						gtx.Constraints.Min.X = 0
						return component.Menu(th, &rc.menuState).Layout(gtx)
					})
				})
				stackChildren = append(stackChildren, menuStackChild)
			}

			dims := material.Clickable(gtx, &rc.btn, func(gtx layout.Context) layout.Dimensions {
				return layout.Stack{}.Layout(gtx, stackChildren...)
			})

			return dims
		})
	}

	var editorTabBar layout.Widget = func(gtx layout.Context) layout.Dimensions {
		cb := material.CheckBox(th, &rp.showSchema, "Schema")
		cb.Size = unit.Dp(16)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// the tabs
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1.0, crEditorTab),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return cb.Layout(gtx)
					}),
				)
			}),
			// The separator line
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if rp.current == nil || rp.activeResources.Size() == 0 {
					return layout.Dimensions{}
				}
				divider := component.Divider(th)
				divider.Top = unit.Dp(0)
				divider.Bottom = unit.Dp(0)
				divider.Fill = th.Palette.ContrastBg
				return divider.Layout(gtx)
			}),
		)
	}

	var crEditorWidget layout.Widget = func(gtx layout.Context) layout.Dimensions {

		if rp.editorBtnDeploy.Clicked(gtx) {
			go func() {
				rp.DeployResource(rp.current)
			}()
		}
		if rp.editorBtnSave.Clicked(gtx) {
			rp.SaveCurrent()
		}
		// The editor area
		if rp.current == nil || rp.activeResources.Size() == 0 {
			rp.crEditor.Editor.SetText("")
			//don't show vertical bar if current is nil
			return layout.UniformInset(unit.Dp(8)).Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return rp.crEditor.Layout(gtx)
				},
			)
		}
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			// the vertial action bar
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(0), Bottom: unit.Dp(0), Left: unit.Dp(0), Right: unit.Dp(0)}.Layout(gtx,
					func(gtx layout.Context) layout.Dimensions {
						return layout.Stack{Alignment: layout.NW}.Layout(gtx,
							// the bar
							layout.Stacked(func(gtx layout.Context) layout.Dimensions {
								barHeight := gtx.Constraints.Max.Y
								barWidth := gtx.Dp(unit.Dp(24))
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
									rp.GetBarButtons(th)...,
								)
							}),
						)
					},
				)
			}),
			// the editor
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(10)).Layout(gtx,
					func(gtx layout.Context) layout.Dimensions {
						return rp.crEditor.Layout(gtx)
					})
			}),
		)
	}

	var fullFieldEditorWidget layout.Widget = func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(6)).Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				return rp.fullEditor.Layout(gtx)
			})
	}

	var crdEditorWidget layout.Widget = func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(6)).Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				return rp.crdEditor.Layout(gtx)
			})
	}

	showFullFields := graphics.ShowFullCr()
	if rp.showSchema.Pressed() {
		rp.showSchema.Update(gtx)
	}
	showSchema := rp.showSchema.Value

	if showFullFields && showSchema {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(editorTabBar),
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				return rp.primarySplit.Layout(gtx,
					crEditorWidget,
					func(gtx layout.Context) layout.Dimensions {
						return rp.bothPanel.Layout(gtx,
							fullFieldEditorWidget,
							crdEditorWidget,
							common.VerticalSplitHandler,
						)
					},
					common.VerticalSplitHandler,
				)
			}),
		)
	}

	if showFullFields {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(editorTabBar),
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				return rp.primarySplit.Layout(gtx,
					crEditorWidget,
					fullFieldEditorWidget,
					common.VerticalSplitHandler,
				)
			}),
		)
	}

	if showSchema {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(editorTabBar),
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				return rp.primarySplit.Layout(gtx,
					crEditorWidget,
					crdEditorWidget,
					common.VerticalSplitHandler,
				)
			}),
		)
	}

	//only the crEditorWidget
	if rp.activeResources.Size() == 0 {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Flexed(1.0, crEditorWidget),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(editorTabBar),
		layout.Flexed(1.0, crEditorWidget),
	)

}

// re-arrange the map to a list of keys and make sure all namespaces
// ids are in the front of the slice (so deployed first)
func ProcessDeployOrder(deployMap map[string]*common.ResourceInstanceAction) []string {
	result := make([]string, len(deployMap))
	index1 := 0
	index2 := len(deployMap) - 1

	for key, inst := range deployMap {
		if inst.Instance.GetSpecApiVer() == "v1/namespaces" {
			result[index1] = key
			index1++
		} else {
			result[index2] = key
			index2--
		}
	}
	return result
}

// Note: this method is called in a go routine
// be careful not to update the ui directly in this method scope
// if app crashes examine this method's call stacks and see
// if somewhere in the path it updates UI directly. If so
// move them to the layout path.
func (rp *ResourcePage) DeployResource(current common.Resource) error {
	appLog := logs.GetLogger(logs.IN_APP_LOGGER_NAME)
	currentId := rp.current.GetId()

	inode := rp.resourceManager.GetNodeMap()[current.GetId()]
	if inode == nil {
		// this could happen with template resource where it's not in the repository
		fmt.Printf("Logging warn current: %v, with logger: %v\n", current.GetName(), logger)
		logger.Warn("Resource is a template", zap.String("Name", current.GetName()))
		if inst, ok := current.(*common.ResourceInstance); ok {
			inode = common.NewResourceNode(inst)
		} else {
			return fmt.Errorf("current is not a ResourceInstance: %T", current)
		}
	}
	resourcesToDeploy, err := rp.deployedResources.LockAndAdd(inode)
	if err != nil {
		appLog.Warn("Failed to deploy resource", zap.String("Name", current.GetName()))
		return err
	}
	if len(resourcesToDeploy) == 0 {
		appLog.Info("No resources to deploy")
		return nil
	}

	orderedResourceToDeploy := ProcessDeployOrder(resourcesToDeploy)

	ctxData, _ := common.GetContextData(common.CONTEXT_LONG_TASK_LIST)
	if taskCtx, ok := ctxData.(*common.LongTasksContext); ok {
		task := taskCtx.AddTask("Deploying resource")

		task.Run = func() {
			finalNs := make(map[string]types.NamespacedName, 0)
			task.Progress = float32(0.1)
			task.Update("Starting")
			task.Step = 0.9 / float32(len(orderedResourceToDeploy))

			logger.Debug("Resources to deploy", zap.Int("count", len(orderedResourceToDeploy)))

			for _, res := range orderedResourceToDeploy {
				toDeploy := resourcesToDeploy[res]
				if ns, err := rp.k8sClient.DeployResource(toDeploy, ""); err != nil {
					logger.Error("Failed to deploy resource", zap.Any("res", res), zap.Error(err))
					task.Failed(err)
					rp.deployedResources.Remove(currentId)
					return
				} else {
					// for a collection
					finalNs[toDeploy.Instance.GetId()] = ns
					//update progress
					task.Update("deployed " + toDeploy.GetName())
				}
			}
			task.Done()
			// release locked res so that deploy button should be enabled again
			rp.deployedResources.Deployed(currentId, finalNs)
		}
		task.Start()
	}
	return nil
}

func (rp *ResourcePage) SaveCurrent() {
	if rp.current != nil {
		rp.current.SetCR(rp.crPanel.Text())
		if rp.current.GetName() == "" {
			if inst, ok := rp.current.(*common.ResourceInstance); ok {
				rp.resourceManager.SaveTemplate(inst)
			} else {
				logger.Warn("Resource is not a ResourceInstance", zap.String("Name", rp.current.GetName()))
			}
		} else {
			rp.resourceManager.SaveResource(rp.current.GetId())
		}
	}
}

func (rp *ResourcePage) GetBarButtons(th *material.Theme) []layout.FlexChild {
	children := make([]layout.FlexChild, 0)
	//disable deploy on the whole repo
	if rp.current != nil {
		if !rp.resourceManager.IsRepo(rp.current.GetId()) {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(10)}.Layout(gtx,
					func(gtx layout.Context) layout.Dimensions {
						// to disable the button use gtx = gtx.Disabled()
						button := component.TipIconButtonStyle{
							Tooltip:         rp.DeployBtnTooltip,
							IconButtonStyle: material.IconButton(th, &rp.editorBtnDeploy, graphics.DeployIcon, "Deploy"),
							State:           &rp.DeployBtnTipArea,
						}

						button.Size = 20
						button.IconButtonStyle.Inset = layout.Inset{Top: 1, Bottom: 1, Left: 1, Right: 1}
						return button.Layout(gtx)
					},
				)
			}))
		}

		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(10)}.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					button := component.TipIconButtonStyle{
						Tooltip:         rp.SaveBtnTooltip,
						IconButtonStyle: material.IconButton(th, &rp.editorBtnSave, graphics.SaveIcon, "Save"),
						State:           &rp.SaveBtnTipArea,
					}

					button.Size = 20
					button.IconButtonStyle.Inset = layout.Inset{Top: 1, Bottom: 1, Left: 1, Right: 1}
					return button.Layout(gtx)
				},
			)
		}))

		// the space separating top buttons and bottom ones
		children = append(children, layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{
				Size: image.Point{
					X: gtx.Constraints.Min.X,
					Y: gtx.Constraints.Min.Y,
				},
			}
		}))

		// add separator
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			divider := component.Divider(th)
			divider.Top = unit.Dp(1)
			divider.Bottom = unit.Dp(1)
			divider.Fill = th.Palette.ContrastBg
			return divider.Layout(gtx)
		}))
	}
	return children
}

// put it into a queue. maybe we should
// check clicked outside tab.Layout (use a for loop over all active resources to do it)
// so we don't need the queue.
func (rp *ResourcePage) Close(rcs ...*ActiveResource) {
	rp.activeResources.pushClose(rcs...)
}
