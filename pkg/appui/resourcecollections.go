package appui

import (
	"fmt"
	"image"
	"slices"
	"strings"

	"gaohoward.tools/k8s/resutil/pkg/common"
	"gaohoward.tools/k8s/resutil/pkg/config"
	"gaohoward.tools/k8s/resutil/pkg/dialogs"
	"gaohoward.tools/k8s/resutil/pkg/graphics"
	"gaohoward.tools/k8s/resutil/pkg/k8sservice"
	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

var resourceCollections *ResourceCollections

// A collection repository is a
// resource, as in either a local dir
// (may be a git repo that supports version)
// or a remote url (https/ssh that may
// speak git). When it's a remote
// the repo may be cloned to local
// The CollectionRepository is just
// a top level Collection with
// info to access the contents
type CollectionRepository struct {
	*common.Collection
}

func (cr *CollectionRepository) CloneRepoForInput(newHolder map[string]common.INode) *CollectionRepository {
	cl := cr.Collection.CloneForInput(cr.Collection, newHolder)
	if col, ok := cl.(*common.Collection); ok {

		return &CollectionRepository{
			col,
		}
	}
	return nil
}

func NewCollectionRepo(name string, pid *string, id *string, config *config.CollectionConfig, path string, holder map[string]common.INode) *CollectionRepository {
	repo := CollectionRepository{
		common.NewCollection(name, pid, id, config, path, holder),
	}
	return &repo
}

type Action int

const (
	AddCollection Action = iota
	AddResource
	AddTemplate
	Remove
	Reload
	Reorder
)

func (a Action) getActionTitle() string {
	return a.getActionName()
}

func (a Action) getActionName() string {
	switch a {
	case AddCollection:
		return "Add Collection"
	case AddResource:
		return "Add Resource"
	case AddTemplate:
		return "Add Template"
	case Remove:
		return "Remove"
	case Reload:
		return "Reload"
	case Reorder:
		return "Re-Ordering"
	default:
		return "Unknown Action"
	}
}

// impl dialogs.DialogControl
type AddResourceDialogControl struct {
	action    Action
	nameInput component.TextField
	typeInput component.TextField

	menuState               component.MenuState
	contextArea             component.ContextArea
	apiResourceClickControl []*ApiResourceControl

	pathLabel material.LabelStyle
	panel     layout.Widget
	tree      *ResourceCollections

	path       string
	id         string
	actionData any

	initFocus bool
}

func (adc *AddResourceDialogControl) Reset(act Action) {

	adc.tree.currentNode = resourceCollections.currentNode
	initPath := ""
	if adc.tree.currentNode != nil {
		initPath = adc.tree.currentNode.GetPath()
		adc.id = adc.tree.currentNode.GetOwnerCollection().GetId()
	}
	if _, ok := adc.tree.currentNode.(*common.ResourceNode); ok {
		_, initPath = common.ExtractNameFromPath(initPath)
	}
	adc.path = initPath
}

// PathSeleted implements common.RepoListener.
func (adc *AddResourceDialogControl) PathSeleted(collectionId string, path string) {
	adc.id = collectionId
	adc.path = path
}

// ResourceSelected implements common.RepoListener.
func (adc *AddResourceDialogControl) ResourceSelected(resourceId string, resPath string) {
	//if resource selected, ignore it. however we may extract the path and find its collection id
	//but still it's confusing. Probably we can hide all resource nodes in the repo tree in dialog
}

func (adc *AddResourceDialogControl) GetTitle() string {
	return adc.action.getActionTitle()
}

func (adc *AddResourceDialogControl) GetWidget(th *material.Theme) layout.Widget {
	return adc.panel
}

func (adc *AddResourceDialogControl) doAddTemplate() error {
	trimdName := strings.TrimSpace(adc.nameInput.Text())
	if trimdName == "" {
		err := fmt.Errorf("name shouldn't be empty")
		adc.nameInput.SetError(err.Error())
		return err
	}
	//check name uniqueness
	node := resourceCollections.FindNode(adc.id)
	if node == nil {
		return fmt.Errorf("cannot find collection %v", adc.id)
	}
	if col, ok := node.(*common.Collection); ok {
		if col.FindDirectResourceByName(trimdName) != nil {
			err := fmt.Errorf("resource name %v already exists", trimdName)
			return err
		}
	}

	inst := adc.actionData.(*common.ResourceInstance)

	trimdPath := strings.TrimSpace(adc.path)
	if trimdPath == "" {
		return fmt.Errorf("need to select a path")
	}

	return resourceCollections.AddNewResourceFromTemplate(adc.id, trimdPath, trimdName, inst)
}

func (adc *AddResourceDialogControl) doAddResource() error {
	trimdName := strings.TrimSpace(adc.nameInput.Text())
	if trimdName == "" {
		err := fmt.Errorf("name shouldn't be empty")
		adc.nameInput.SetError(err.Error())
		return err
	}
	//check name uniqueness
	node := resourceCollections.FindNode(adc.id)
	if node == nil {
		return fmt.Errorf("cannot find collection %v", adc.id)
	}
	if col, ok := node.(*common.Collection); ok {
		if col.FindDirectResourceByName(trimdName) != nil {
			err := fmt.Errorf("resource name %v already exists", trimdName)
			return err
		}
	}

	trimdType := strings.TrimSpace(adc.typeInput.Text())

	var ok bool
	var apiVer string

	if ok, apiVer = k8sservice.IsTypeSupported(trimdType); !ok {
		return fmt.Errorf("type is not supported %v", trimdType)
	}

	trimdPath := strings.TrimSpace(adc.path)
	if trimdPath == "" {
		return fmt.Errorf("need to select a path")
	}
	return resourceCollections.AddNewResource(adc.id, trimdPath, trimdName, apiVer)
}

func (adc *AddResourceDialogControl) doAddCollection() error {

	trimdPath := strings.TrimSpace(adc.path)
	if trimdPath == "" {
		return fmt.Errorf("need to select a path")
	}

	trimdName := strings.TrimSpace(adc.nameInput.Text())
	if trimdName == "" {
		err := fmt.Errorf("name shouldn't be empty")
		adc.nameInput.SetError(err.Error())
		return err
	}

	//check spaces in trimed name
	if strings.Contains(trimdName, " ") {
		err := fmt.Errorf("name shouldn't contain spaces")
		adc.nameInput.SetError(err.Error())
		return err
	}

	//check composite collections like /col1/col2/...
	parts := strings.Split(trimdName, "/")
	effectiveParts := make([]string, 0)
	for _, p := range parts {
		tp := strings.TrimSpace(p)
		if tp != "" {
			effectiveParts = append(effectiveParts, tp)
		}
	}

	// make sure the first col doesn't exist
	node := resourceCollections.FindNode(adc.id)
	if node == nil {
		return fmt.Errorf("cannot find collection %v", adc.id)
	}

	if col, ok := node.(*common.Collection); ok {
		for _, val := range col.GetChildren() {
			if val.GetName() == effectiveParts[0] {
				err := fmt.Errorf("resource name %v already exists", effectiveParts[0])
				return err
			}
		}
	}

	//now create collections
	return resourceCollections.AddNewCollections(adc.id, adc.path, effectiveParts)
}

func (adc *AddResourceDialogControl) Apply() error {
	switch adc.action {
	case AddResource:
		return adc.doAddResource()
	case AddCollection:
		return adc.doAddCollection()
	case AddTemplate:
		return adc.doAddTemplate()
	case Reorder:
		return adc.doReorder()
	}
	return fmt.Errorf("unsupported action %v", adc.action)
}

func (adc *AddResourceDialogControl) doReorder() error {
	if reorderPanel, ok := adc.actionData.(*ReorderPanel); ok {
		roMap := reorderPanel.GetReorderMap()
		if len(roMap) == 0 {
			return nil
		}

		for k, v := range roMap {
			resMap := resourceCollections.GetNodeMap()
			if node, ok := resMap[k]; ok {
				if col, ok := node.(*common.Collection); ok {
					for i, itm := range v.currentResources {
						col.GetResourceBag().SetInstanceAt(i, itm)
					}
					col.GetResourceBag().Save(col.GetPath())
					col.Reload("")
				}
			}
		}
	}
	return nil
}

func (adc *AddResourceDialogControl) Cancel() {

}

func (a *AddResourceDialogControl) RequestFocusOnce(gtx layout.Context) {
	for {
		event, ok := gtx.Event(key.FocusFilter{Target: &a})
		if !ok {
			break
		}
		switch event.(type) {
		case key.FocusEvent:
			if !a.initFocus {
				gtx.Execute(key.FocusCmd{Tag: &a.nameInput.Editor})
				a.initFocus = true
			}
		default:
			gtx.Execute(key.FocusCmd{Tag: nil})
		}
	}
}

func NewAddCollectionDialogControl(th *material.Theme) *AddResourceDialogControl {
	control := &AddResourceDialogControl{
		action: AddCollection,
		path:   "",
	}
	control.tree = resourceCollections.CloneForInput(th)
	control.tree.AddListener(control)

	control.nameInput.SingleLine = true
	control.nameInput.Editor.Submit = true

	control.panel = func(gtx layout.Context) layout.Dimensions {

		control.setupLocationLabel(th)

		control.RequestFocusOnce(gtx)

		// name, type, and location(repo path)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return control.nameInput.Layout(gtx, th, "Name of the collection")
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					// a label and the tree
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return control.getPathWidget()(gtx)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						//tree
						return control.tree.LayoutForInput(gtx, th)
					}),
				)
			}),
		)
	}
	control.Reset(AddCollection)
	return control
}

const (
	ToTopButtonIndex    = 0
	MoveUpButtonIndex   = 1
	MoveDownButtonIndex = 2
	ToBottomButtonIndex = 3
	RestoreButtonIndex  = 4
)

type MoveControl struct {
	Top     widget.Clickable
	Up      widget.Clickable
	Down    widget.Clickable
	Bottom  widget.Clickable
	Restore widget.Clickable

	btnList  []*widget.Clickable
	iconList []*widget.Icon

	ButtonList layout.List
}

func (mc *MoveControl) ShouldButtonBeDisabled(btIndex int, source *ReorderPanel) bool {
	if source.currentTarget == -1 {
		return true
	}
	// if current is at top, toTop button and moveUp should be disabled
	if source.currentTarget == 0 && (btIndex == ToTopButtonIndex || btIndex == MoveUpButtonIndex) {
		return true
	}
	// if current is at bottom, toBottom and movcDown should be disabled
	if source.currentTarget == source.currentItems.Size()-1 && (btIndex == ToBottomButtonIndex || btIndex == MoveDownButtonIndex) {
		return true
	}
	return false
}

func (mc *MoveControl) Reorder(btIdx int, source *ReorderPanel) {
	switch btIdx {
	case ToTopButtonIndex:
		source.MoveCurrentToTop()
	case ToBottomButtonIndex:
		source.MoveCurrentToBottom()
	case MoveUpButtonIndex:
		source.MoveCurrentUp()
	case MoveDownButtonIndex:
		source.MoveCurrentDown()
	case RestoreButtonIndex:
		source.RestoreOrder()
	}
}

func (mc *MoveControl) Layout(gtx layout.Context, th *material.Theme, source *ReorderPanel) layout.Dimensions {

	return mc.ButtonList.Layout(gtx, 5, func(gtx layout.Context, index int) layout.Dimensions {

		if mc.ShouldButtonBeDisabled(index, source) {
			gtx = gtx.Disabled()
		}

		btn := mc.btnList[index]

		if btn.Clicked(gtx) {
			mc.Reorder(index, source)
		}
		icon := mc.iconList[index]
		ibtn := material.IconButton(th, btn, icon, "mv")
		ibtn.Inset.Top = 0
		ibtn.Inset.Bottom = 0
		ibtn.Inset.Left = 0
		ibtn.Inset.Right = 0
		ibtn.Size = unit.Dp(20)

		return layout.Inset{Top: 0, Bottom: 1, Left: unit.Dp(8), Right: unit.Dp(16)}.Layout(gtx, ibtn.Layout)
	})
}

type ResourceItems struct {
	currentResources   []*common.ResourceNode
	resourceClickables []widget.Clickable
}

func (r *ResourceItems) hasOrderChanged() bool {
	for i, item := range r.currentResources {
		if *item.Instance.Order != i {
			return true
		}
	}
	return false
}

func (r *ResourceItems) RestoreOrder(current int) int {
	newCurrent := r.currentResources[current].Instance.Order
	slices.SortFunc(r.currentResources, func(a, b *common.ResourceNode) int {
		if *a.Instance.Order > *b.Instance.Order {
			return 1
		}
		if *a.Instance.Order < *b.Instance.Order {
			return -1
		}
		return 0
	})
	return *newCurrent
}

func (r *ResourceItems) MoveCurrent(fromIndex, toIndex int) int {
	common.ReorderSlice(r.currentResources, fromIndex, toIndex)
	return toIndex
}

func (r *ResourceItems) Size() int {
	return len(r.currentResources)
}

func (r *ResourceItems) Get(index int) (*common.ResourceNode, *widget.Clickable) {
	return r.currentResources[index], &r.resourceClickables[index]
}

type ReorderPanel struct {
	owner *AddResourceDialogControl
	// col id -> new orders (for example if a col has 3 resources [2, 1, 0] means res 2 becomes
	// the first, res 0 becomes the 3rd, res 1 unchanged.)
	reorderMap map[string]*ResourceItems

	currentItems  *ResourceItems
	currentTarget int

	moveControl *MoveControl

	resourceList layout.List
}

func (rp *ReorderPanel) GetReorderMap() map[string]*ResourceItems {
	prunedMap := make(map[string]*ResourceItems)
	for k, v := range rp.reorderMap {
		if v.hasOrderChanged() {
			prunedMap[k] = v
		}
	}
	return prunedMap
}

func (rp *ReorderPanel) MoveCurrentToTop() {
	rp.currentTarget = rp.currentItems.MoveCurrent(rp.currentTarget, 0)
}

func (rp *ReorderPanel) MoveCurrentToBottom() {
	rp.currentTarget = rp.currentItems.MoveCurrent(rp.currentTarget, rp.currentItems.Size()-1)
}

func (rp *ReorderPanel) MoveCurrentUp() {
	rp.currentTarget = rp.currentItems.MoveCurrent(rp.currentTarget, rp.currentTarget-1)
}

func (rp *ReorderPanel) MoveCurrentDown() {
	rp.currentTarget = rp.currentItems.MoveCurrent(rp.currentTarget, rp.currentTarget+1)
}

func (rp *ReorderPanel) RestoreOrder() {
	rp.currentTarget = rp.currentItems.RestoreOrder(rp.currentTarget)
}

func (rp *ReorderPanel) SetCurrentCollection(col *common.Collection) {

	if c, ok := rp.reorderMap[col.Id]; ok {
		rp.currentItems = c
		return
	}

	rp.currentTarget = -1

	items := &ResourceItems{
		currentResources:   make([]*common.ResourceNode, len(col.GetResourceBag().ResourceNodes)),
		resourceClickables: make([]widget.Clickable, len(col.GetResourceBag().ResourceNodes)),
	}

	copy(items.currentResources, col.GetResourceBag().ResourceNodes)
	// for i, rn := range col.GetResourceBag().ResourceNodes {
	// 	items.currentResources[i] = rn
	// }

	rp.currentItems = items
	rp.reorderMap[col.Id] = items
}

func (rp *ReorderPanel) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if rp.owner.tree.currentNode != nil {
		if col, ok := rp.owner.tree.currentNode.(*common.Collection); ok {

			rp.SetCurrentCollection(col)

			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					// The control bar (change order)
					return rp.moveControl.Layout(gtx, th, rp)
				}),
				layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
					// The resource order list
					if rp.currentItems.Size() > 0 {
						return rp.resourceList.Layout(gtx, rp.currentItems.Size(), func(gtx layout.Context, index int) layout.Dimensions {
							thisRes, thisBtn := rp.currentItems.Get(index)
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{
										Top:    unit.Dp(1),
										Bottom: unit.Dp(1),
										Left:   0,
										Right:  unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return material.Label(th, unit.Sp(16), fmt.Sprintf("%2d", index)).Layout(gtx)
									})
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if thisBtn.Clicked(gtx) {
										rp.currentTarget = index
									}
									return material.Clickable(gtx, thisBtn, func(gtx layout.Context) layout.Dimensions {
										label := material.Label(th, unit.Sp(16), thisRes.GetLabel())
										label.Font.Weight = font.Normal
										if rp.currentTarget != -1 {
											res, _ := rp.currentItems.Get(rp.currentTarget)
											if res == thisRes {
												label.Font.Weight = font.Bold
											}
										}
										return label.Layout(gtx)
									})
								}),
							)
						})
					}
					//no resources
					return material.H6(th, "No resources").Layout(gtx)
				}),
			)
		}
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return material.H6(th, "The current selection is not a collection").Layout(gtx)
		})
	}
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return material.H6(th, "Please select a collection").Layout(gtx)
	})
}

func (c *AddResourceDialogControl) GetWidgetForReorder() *ReorderPanel {
	mvCtl := &MoveControl{}
	mvCtl.ButtonList.Axis = layout.Vertical
	mvCtl.btnList = []*widget.Clickable{
		&mvCtl.Top,
		&mvCtl.Up,
		&mvCtl.Down,
		&mvCtl.Bottom,
		&mvCtl.Restore,
	}
	mvCtl.iconList = []*widget.Icon{
		graphics.ToTopIcon,
		graphics.MoveUpIcon,
		graphics.MoveDownIcon,
		graphics.ToBottomIcon,
		graphics.RestoreIcon,
	}

	rp := &ReorderPanel{
		owner:         c,
		reorderMap:    make(map[string]*ResourceItems),
		currentItems:  nil,
		moveControl:   mvCtl,
		currentTarget: -1,
	}

	rp.resourceList.Axis = layout.Vertical

	if c.tree.currentNode != nil {
		if col, ok := c.tree.currentNode.(*common.Collection); ok {
			rp.SetCurrentCollection(col)
		}
	}
	return rp
}

func NewReorderDialogControl(th *material.Theme) *AddResourceDialogControl {
	control := &AddResourceDialogControl{
		action: Reorder,
		path:   "",
	}
	control.tree = resourceCollections.CloneForInput(th)
	control.tree.AddListener(control)

	reorderComp := control.GetWidgetForReorder()
	control.actionData = reorderComp

	control.panel = func(gtx layout.Context) layout.Dimensions {

		control.setupLocationLabel(th)

		// name, type, and location(repo path)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Flexed(0.4, func(gtx layout.Context) layout.Dimensions {
				return reorderComp.Layout(gtx, th)
			}),
			layout.Flexed(0.6, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					// a label and the tree
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return control.getPathWidget()(gtx)
					}),
					layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
						//tree
						return control.tree.LayoutForInput(gtx, th)
					}),
				)
			}),
		)
	}
	control.Reset(AddCollection)
	return control
}

type ApiResourceControl struct {
	groupVersion string
	apiName      string
	clickable    widget.Clickable
}

func NewAddTemplateDialogControl(th *material.Theme, actionData any) *AddResourceDialogControl {

	control := &AddResourceDialogControl{
		action:     AddTemplate,
		path:       "",
		actionData: actionData,
	}

	inst := actionData.(*common.ResourceInstance)

	control.tree = resourceCollections.CloneForInput(th)
	control.tree.AddListener(control)
	control.nameInput.SingleLine = true
	control.typeInput.ReadOnly = true
	control.typeInput.SetText(inst.GetSpecApiVer())

	control.setupLocationLabel(th)

	control.panel = func(gtx layout.Context) layout.Dimensions {

		// name, type, and location(repo path)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return control.nameInput.Layout(gtx, th, "Name of the resource")
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Stack{}.Layout(gtx,
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return control.typeInput.Layout(gtx, th, "")
					}),
				)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					// a label and the tree
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return control.getPathWidget()(gtx)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						//tree
						return control.tree.LayoutForInput(gtx, th)
					}),
				)
			}),
		)
	}
	control.Reset(AddTemplate)
	return control

}

func (adc *AddResourceDialogControl) loadMenus(th *material.Theme) {
	if adc.action == AddResource {

		k8sService := k8sservice.GetK8sService()

		apiResources := k8sService.FetchAllApiResources(false)

		apiResourceMenuItems := make([]func(gtx layout.Context) layout.Dimensions, 0)

		adc.apiResourceClickControl = make([]*ApiResourceControl, 0)

		if apiResources != nil {
			for _, apiGroup := range apiResources.ResList {
				for _, api := range apiGroup.APIResources {
					apiControl := &ApiResourceControl{
						groupVersion: apiGroup.GroupVersion,
						apiName:      api.Name,
					}
					adc.apiResourceClickControl = append(adc.apiResourceClickControl, apiControl)
				}
			}
		} else {
			for k := range common.ApiVersionMap {
				if gv := k.ToGroupVersion(); gv != "" {
					apiControl := &ApiResourceControl{
						groupVersion: k.ToGroupVersion(),
						apiName:      k.ToApiName(),
					}
					adc.apiResourceClickControl = append(adc.apiResourceClickControl, apiControl)
				}
			}
		}
		//sort apiResourceClickControl
		slices.SortStableFunc(adc.apiResourceClickControl, func(a, b *ApiResourceControl) int {
			if a.apiName < b.apiName {
				return -1
			} else if a.apiName > b.apiName {
				return 1
			}
			return 0
		})

		for _, c := range adc.apiResourceClickControl {
			apiResourceMenuItems = append(apiResourceMenuItems,
				component.MenuItem(th, &c.clickable, c.apiName).Layout)
		}

		adc.menuState = component.MenuState{
			Options: apiResourceMenuItems,
		}
	}
}

func (c *AddResourceDialogControl) setupLocationLabel(th *material.Theme) {
	c.pathLabel = material.H6(th, c.path)
	c.pathLabel.TextSize = unit.Sp(14)
}

func (c *AddResourceDialogControl) getPathWidget() layout.Widget {
	pathWidget := func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(14), Bottom: unit.Dp(2), Left: unit.Dp(2), Right: 0}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				return c.pathLabel.Layout(gtx)
			},
		)
	}
	return pathWidget
}

func NewAddResourceDialogControl(th *material.Theme) *AddResourceDialogControl {
	control := &AddResourceDialogControl{
		action: AddResource,
		path:   "",
	}

	control.tree = resourceCollections.CloneForInput(th)
	control.tree.AddListener(control)

	control.nameInput.SingleLine = true
	control.typeInput.SingleLine = true

	control.loadMenus(th)

	control.panel = func(gtx layout.Context) layout.Dimensions {

		control.setupLocationLabel(th)

		for _, apiControl := range control.apiResourceClickControl {
			if apiControl.clickable.Clicked(gtx) {
				control.typeInput.SetText(apiControl.groupVersion + "/" + apiControl.apiName)
			}
		}

		control.RequestFocusOnce(gtx)

		// name, type, and location(repo path)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return control.nameInput.Layout(gtx, th, "Name of the resource")
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Stack{}.Layout(gtx,
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return control.typeInput.Layout(gtx, th, "Type (e.g. pod), right-click to show all types")
					}),
					layout.Expanded(func(gtx layout.Context) layout.Dimensions {
						return control.contextArea.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.X = 0
							return component.Menu(th, &control.menuState).Layout(gtx)
						})
					}),
				)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					// a label and the tree
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return control.getPathWidget()(gtx)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						//tree
						return control.tree.LayoutForInput(gtx, th)
					}),
				)
			}),
		)
	}
	control.Reset(AddResource)
	return control
}

type CollectionActionHandler struct {
	Owner *ResourceCollections
	// controlAddCol  *AddResourceDialogControl
	// constrolAddRes *AddResourceDialogControl
	dialog *dialogs.InputDialog
}

// pos is the current selection: ResourceNode or Collection
// if it's resourceNode get its parent collection.
func (cah *CollectionActionHandler) handleAction(gtx layout.Context, th *material.Theme, pos common.INode, action Action, actionData any) {
	// try do decide the target collection
	var err error
	if action == Reload {
		// reload the resource
		err = pos.Reload(pos.GetPath())
		cah.Owner.ResourceUpdated(pos)
		if err == nil {
			return
		}
	}

	if action == Remove {
		resourceCollections.RemoveCurrentResource()
		return
	}

	activeControl := cah.GetControlForAction(th, action, actionData)
	if activeControl == nil {
		return
	}

	cah.dialog = dialogs.NewInputDialog(activeControl.GetTitle(), activeControl)

	dialogs.RegisterDialog(cah.dialog) // this add to layout loop
	cah.dialog.Show(gtx)
}

func (cah *CollectionActionHandler) GetControlForAction(th *material.Theme, action Action, actionData any) *AddResourceDialogControl {
	switch action {
	case AddCollection:
		return NewAddCollectionDialogControl(th)
	case AddResource:
		return NewAddResourceDialogControl(th)
	case AddTemplate:
		return NewAddTemplateDialogControl(th, actionData)
	case Reorder:
		return NewReorderDialogControl(th)
	}
	return nil
}

type ResourceCollections struct {
	Name              string
	list              widget.List
	menuState         component.MenuState
	noSelectMenuState component.MenuState
	addResourceBtn    widget.Clickable
	addCollectionBtn  widget.Clickable
	removeBtn         widget.Clickable
	reloadBtn         widget.Clickable
	reorderBtn        widget.Clickable
	noSelectBtn       widget.Clickable
	menuContextArea   component.ContextArea

	currentNode common.INode

	// to support multiple repos
	// make this an array. each
	// represents a repository (local or remote)
	repos []*CollectionRepository
	// all nodes keyed by their ids, used for quick search
	nodeMap      map[string]common.INode
	ResourcePage *ResourcePage

	ActionHandler *CollectionActionHandler
	listener      common.RepoListener
}

func (c *ResourceCollections) IsRepo(id string) bool {
	if n, ok := c.nodeMap[id]; ok {
		return n.IsRoot()
	}
	return false
}

// SaveTemplate implements common.ResourceManager.
func (c *ResourceCollections) SaveTemplate(current *common.ResourceInstance) {
	newInst := current.Clone()

	// launch the addresource dialog
	common.SetContextBool(common.CONTEXT_SAVE_TEMPLATE, true, newInst)
}

func (c *ResourceCollections) GetNodeMap() map[string]common.INode {
	return c.nodeMap
}

func (c *ResourceCollections) SaveResource(resId string) {
	if node, ok := c.nodeMap[resId]; ok {
		node.Save("", false)
	}
}

func (c *ResourceCollections) RemoveCurrentResource() {
	// remove a resource/collection
	if c.currentNode != nil {
		isRepo := false
		for _, rpo := range c.repos {
			if rpo.GetId() == c.currentNode.GetId() {
				isRepo = true
			}
		}
		if isRepo {
			return
		}
		parent := c.currentNode.GetParent()
		c.ResourcePage.RemoveRefs(c.currentNode.GetId(), c.nodeMap)
		c.currentNode.Remove()

		c.currentNode = nil

		parent.Reload("")
	}
}

func (c *ResourceCollections) ResourceUpdated(pos common.INode) {
	c.ResourcePage.UpdateActiveContents(pos.GetAllResources(), c.nodeMap)
}

func (c *ResourceCollections) handlContextActions(gtx layout.Context, th *material.Theme) {
	value, data, err := common.GetContextBool(common.CONTEXT_SAVE_TEMPLATE)
	if err != nil {
		logger.Warn("Error getting context", zap.Error(err))
	}
	if value {
		//think of an api to just set the value and leave the extra data unchanged.
		common.SetContextBool(common.CONTEXT_SAVE_TEMPLATE, false, data)
		if inst, ok := data.(*common.ResourceInstance); ok {
			c.ActionHandler.handleAction(gtx, th, c.currentNode, AddTemplate, inst)
		}
	}
}

func (c *ResourceCollections) handleMenuActions(gtx layout.Context, th *material.Theme) {
	if c.addCollectionBtn.Clicked(gtx) {
		c.ActionHandler.handleAction(gtx, th, c.currentNode, AddCollection, nil)
	}
	if c.addResourceBtn.Clicked(gtx) {
		c.ActionHandler.handleAction(gtx, th, c.currentNode, AddResource, nil)
	}
	if c.removeBtn.Clicked(gtx) {
		c.ActionHandler.handleAction(gtx, th, c.currentNode, Remove, nil)
	}
	if c.reloadBtn.Clicked(gtx) {
		c.ActionHandler.handleAction(gtx, th, c.currentNode, Reload, nil)
		c.ResourcePage.Activate(c.currentNode.GetId())
	}
	if c.reorderBtn.Clicked(gtx) {
		c.ActionHandler.handleAction(gtx, th, c.currentNode, Reorder, nil)
		c.ResourcePage.Activate(c.currentNode.GetId())
	}
}

func (c *ResourceCollections) Load() []error {
	var errs []error
	for _, r := range c.repos {
		err := r.Load("")
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (c *ResourceCollections) Save() []error {

	var errs []error = make([]error, 0)
	for _, r := range c.repos {
		err := r.Save("", false)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (c *ResourceCollections) init(th *material.Theme, holder map[string]common.INode) []error {

	c.noSelectMenuState = component.MenuState{
		Options: []func(gtx component.C) component.D{
			func(gtx component.C) component.D {
				return common.ItemFunc(th, gtx, &c.noSelectBtn, "Please select an item", nil)
			},
		},
	}

	c.menuState = component.MenuState{
		Options: []func(gtx component.C) component.D{
			func(gtx component.C) component.D {
				return common.ItemFunc(th, gtx, &c.addResourceBtn, "Add Resource", graphics.AddToResourceIcon)
			},
			func(gtx component.C) component.D {
				return common.ItemFunc(th, gtx, &c.addCollectionBtn, "Add Collection", graphics.AddToCollectionIcon)
			},
			func(gtx component.C) component.D {
				return common.ItemFunc(th, gtx, &c.removeBtn, "Remove", graphics.DeleteIcon)
			},
			func(gtx component.C) component.D {
				return common.ItemFunc(th, gtx, &c.reloadBtn, "Reload", graphics.ReloadIcon)
			},
			func(gtx component.C) component.D {
				return common.ItemFunc(th, gtx, &c.reorderBtn, "Reorder", graphics.ReorderIcon)
			},
		},
	}

	c.ActionHandler = NewCollectionActionHandler(th, c)

	repoPaths, err := config.GetCollectionRepos()

	if err != nil {
		logger.Error("failed getting col repos", zap.Error(err))
		return []error{err}
	}

	c.nodeMap = holder
	for _, r := range repoPaths {
		name, _ := common.ExtractNameFromPath(r)
		cfg := &config.CollectionConfig{
			CollectionConfigurable: config.CollectionConfigurable{
				Description: "Repository at " + r,
			},
		}
		col := NewCollectionRepo(name, nil, nil, cfg, r, holder)
		c.repos = append(c.repos, col)
	}

	return c.Load()
}

func NewCollectionActionHandler(th *material.Theme, owner *ResourceCollections) *CollectionActionHandler {
	handler := CollectionActionHandler{
		Owner: owner,
	}
	return &handler
}

func (col *ResourceCollections) LayoutCollectionsForInput(gtx layout.Context, th *material.Theme, node *common.Collection) layout.Dimensions {
	if node.GetClickable().Clicked(gtx) {
		col.currentNode = node
		col.listener.PathSeleted(node.GetId(), node.GetPath())
	}

	children := make([]layout.FlexChild, 0, 10)
	if len(node.GetChildren()) > 0 {
		chdrn := node.GetChildren()
		for _, child := range chdrn {
			children = append(children, layout.Rigid(
				func(gtx layout.Context) layout.Dimensions {
					return col.LayoutCollectionsForInput(gtx, th, child)
				}))
		}
	}

	// dont list resources
	return component.SimpleDiscloser(th, node.GetDiscloserState()).Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return material.Clickable(gtx, node.GetClickable(), func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					flatBtnText := material.Body1(th, node.GetName())
					if col.currentNode != nil && node.GetId() == col.currentNode.GetId() {
						flatBtnText.Font.Weight = font.Bold
					} else {
						flatBtnText.Font.Weight = font.Normal
					}
					return layout.Center.Layout(gtx, flatBtnText.Layout)
				})
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
}

// Layout a collection and its children.
func (col *ResourceCollections) LayoutCollections(gtx layout.Context, th *material.Theme, node *common.Collection) layout.Dimensions {
	// check if the current tab on resource page is different
	if col.ResourcePage.current != nil {
		node := col.FindNode(col.ResourcePage.current.GetId())
		if node != nil {
			col.currentNode = node
		}
	}
	if node.GetClickable().Clicked(gtx) {
		col.currentNode = node
		col.ResourcePage.AddActiveResource(common.NewResourceCollection(node), true)
	}

	children := make([]layout.FlexChild, 0, 10)
	if len(node.GetChildren()) > 0 {
		chdrn := node.GetChildren()
		for _, child := range chdrn {
			children = append(children, layout.Rigid(
				func(gtx layout.Context) layout.Dimensions {
					return col.LayoutCollections(gtx, th, child)
				}))
		}
	}

	// now list resources
	bag := node.GetResourceBag()
	if len(bag.ResourceNodes) > 0 {
		for _, res := range bag.ResourceNodes {
			//			logger.Info("---layout res", zap.String("name", res.GetName()), zap.Int("order", *res.Instance.Order))
			if res.GetClickable().Clicked(gtx) {
				col.currentNode = res
				col.ResourcePage.AddActiveResource(res.Instance, true)
			}
			children = append(children, layout.Rigid(
				func(gtx layout.Context) layout.Dimensions {
					nodeComp := material.Clickable(gtx, res.GetClickable(), func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(1)).Layout(gtx,
							func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										gtx.Constraints.Max = image.Point{X: 16, Y: 16}
										color := common.COLOR.Gray
										if col.currentNode != nil && res.GetId() == col.currentNode.GetId() {
											color = common.COLOR.Black
										}
										return graphics.ResourceIcon.Layout(gtx, color)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return layout.Spacer{Width: unit.Dp(4)}.Layout(gtx)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										flatBtnText := material.Body1(th, res.Instance.GetLabel())
										if col.currentNode != nil && res.GetId() == col.currentNode.GetId() {
											flatBtnText.Font.Weight = font.Bold
										} else {
											flatBtnText.Font.Weight = font.Normal
										}
										return layout.W.Layout(gtx, flatBtnText.Layout)
									}),
								)
							},
						)
					})
					return nodeComp
				}))
		}
	}

	return component.SimpleDiscloser(th, node.GetDiscloserState()).Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return material.Clickable(gtx, node.GetClickable(), func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					flatBtnText := material.Body1(th, node.GetName())
					if col.currentNode != nil && node.GetId() == col.currentNode.GetId() {
						flatBtnText.Font.Weight = font.Bold
					} else {
						flatBtnText.Font.Weight = font.Normal
					}
					return layout.Center.Layout(gtx, flatBtnText.Layout)
				})
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
}

// case: user activate a tab and need to locate and highlight the item
// in collections tree
func (c *ResourceCollections) FindNode(resId string) common.INode {
	if n, ok := c.nodeMap[resId]; ok {
		return n
	}
	return nil
}

func (c *ResourceCollections) AddListener(listener common.RepoListener) {
	c.listener = listener
}

func (col *ResourceCollections) LayoutForInput(gtx layout.Context, th *material.Theme) layout.Dimensions {
	col.list.Axis = layout.Vertical
	dim := material.List(th, &col.list).Layout(gtx, len(col.repos), func(gtx layout.Context, index int) layout.Dimensions {
		return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return col.LayoutCollectionsForInput(gtx, th, col.repos[index].Collection)
		})
	})
	return dim
}

func (col *ResourceCollections) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	col.handleMenuActions(gtx, th)

	col.handlContextActions(gtx, th)

	col.list.Axis = layout.Vertical
	dim := material.List(th, &col.list).Layout(gtx, len(col.repos), func(gtx layout.Context, index int) layout.Dimensions {
		return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return col.LayoutCollections(gtx, th, col.repos[index].Collection)
		})
	})
	dim2 := layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return dim
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return col.menuContextArea.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Point{}
				if col.currentNode == nil {
					return component.Menu(th, &col.noSelectMenuState).Layout(gtx)
				}
				return component.Menu(th, &col.menuState).Layout(gtx)
			})
		}),
	)
	return dim2
}

// add a new resource
func (col *ResourceCollections) AddNewResource(id string, newPath string, newName string, newApiVer string) error {
	if c, ok := col.nodeMap[id]; ok {
		if container, ok := c.(*common.Collection); ok {

			instance, err := k8sservice.NewInstance(newApiVer, newName, len(container.GetResourceBag().ResourceNodes))
			if err != nil {
				return err
			}

			resNode := container.AddResource(instance)
			resNode.Save("", false)
			col.ResourcePage.AddActiveResource(instance, true)
		} else {
			logger.Error("Something wrong as it tries to create a resource under a resource node")
		}
		return nil
	} else {
		return fmt.Errorf("collection not exists: %v", id)
	}
}

func (col *ResourceCollections) AddNewResourceFromTemplate(id string, newPath string, newName string, inst *common.ResourceInstance) error {
	if c, ok := col.nodeMap[id]; ok {
		if container, ok := c.(*common.Collection); ok {
			inst.SetId(uuid.New().String())
			inst.SetName(newName)
			inst.Label = newName
			*inst.Order = len(container.GetResourceBag().ResourceNodes)

			resNode := container.AddResource(inst)
			resNode.Save("", false)
			col.ResourcePage.AddActiveResource(inst, true)
			return nil
		}
		return fmt.Errorf("not a container: %v", c)
	}
	return fmt.Errorf("repository not exists: %v", id)
}

func (col *ResourceCollections) AddNewCollections(pid string, path string, effectiveParts []string) error {
	currentPid := pid
	currentPath := path
	var err error
	for _, p := range effectiveParts {
		currentPid, currentPath, err = col.AddNewCollection(currentPid, currentPath, p)
		if err != nil {
			return fmt.Errorf("failed to create collection %v with error %v", p, err)
		}
	}
	return nil
}

// add a new collection
func (col *ResourceCollections) AddNewCollection(containerId string, containerPath string, newCollectionName string) (string, string, error) {
	if c, ok := col.nodeMap[containerId]; ok {
		if container, ok := c.(*common.Collection); ok {
			newCol := container.NewChild(newCollectionName, &config.CollectionConfig{
				CollectionConfigurable: config.CollectionConfigurable{
					Description: "This is a collection",
					Properties: []config.NamedValue{
						{
							Name:  "namespace",
							Value: "default",
						},
					},
				},
			})
			newCol.Save("", false)
			col.ResourcePage.AddActiveResource(common.NewResourceCollection(newCol), true)
			return newCol.Id, newCol.GetPath(), nil
		}
		return "", "", fmt.Errorf("failed to create collection, parent is not a collection! %v", c.GetName())
	} else {
		return "", "", fmt.Errorf("repository not exists: %v", containerId)
	}
}

func (rc *ResourceCollections) CloneForInput(th *material.Theme) *ResourceCollections {
	newResCol := &ResourceCollections{
		Name: "clone",
		//no resource page
	}
	newResCol.nodeMap = make(map[string]common.INode)

	newResCol.repos = make([]*CollectionRepository, 0)
	for _, r := range rc.repos {
		nr := r.CloneRepoForInput(newResCol.nodeMap)
		newResCol.repos = append(newResCol.repos, nr)
	}

	return newResCol
}

func GetResourceCollections(page *ResourcePage, th *material.Theme) (*ResourceCollections, []error) {
	var err []error
	if resourceCollections == nil {
		holder := make(map[string]common.INode)
		resourceCollections = &ResourceCollections{
			ResourcePage: page,
		}
		resourceCollections.ResourcePage.SetResourceManager(resourceCollections)
		err = resourceCollections.init(th, holder)
	}
	return resourceCollections, err
}
