package panels

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"gaohoward.tools/k8s/resutil/pkg/common"
	"gaohoward.tools/k8s/resutil/pkg/graphics"
	"gaohoward.tools/k8s/resutil/pkg/k8sservice"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type ResourceDetail struct {
	detailName string
	clickable  widget.Clickable
	label      material.LabelStyle
	item       *unstructured.Unstructured
	isSelected bool
}

func NewDetail(th *material.Theme, name string, item *unstructured.Unstructured) *ResourceDetail {
	d := &ResourceDetail{
		detailName: name,
		label:      material.H6(th, name),
		item:       item,
	}
	d.label.TextSize = unit.Sp(16)
	return d
}

// GetClickable implements ResourceDetail.
func (r *ResourceDetail) GetClickable() *widget.Clickable {
	return &r.clickable
}

// GetLabel implements ResourceDetail.
func (r *ResourceDetail) GetLabel() layout.Widget {
	if r.isSelected {
		r.label.Color = common.COLOR.Blue()
		r.label.Font.Weight = font.Bold
	} else {
		r.label.Color = common.COLOR.Black()
		r.label.Font.Weight = font.Normal
	}
	return r.label.Layout
}

func (r *ResourceDetail) SetSelected(state bool) {
	r.isSelected = state
}

type DescribeDetail struct {
	*ResourceDetail
	Content       string
	contentEditor *common.ReadOnlyEditor
}

func NewDescribeDetail(th *material.Theme, item *unstructured.Unstructured) common.IResourceDetail {
	d := &DescribeDetail{
		ResourceDetail: NewDetail(th, "describe", item),
	}

	d.contentEditor = common.NewReadOnlyEditor(th, "describe", 16, nil)

	d.contentEditor.SetText(d.getDescribeContent())

	return d
}

func (dd *DescribeDetail) getDescribeContent() *string {
	service := k8sservice.GetK8sService()
	var content string
	var err error
	if service.IsValid() {
		content, err = service.GetDescribeFor(dd.item)
		if err != nil {
			content = err.Error()
		}
	} else {
		content = fmt.Sprintf("unable to get describe name: %s, ns: %s", dd.item.GetName(), dd.item.GetNamespace())
	}
	return &content
}

func (dd *DescribeDetail) GetContent() layout.Widget {
	return dd.contentEditor.Layout
}

func (dd *DescribeDetail) Changed() bool {
	return false
}

type YamlDetail struct {
	*ResourceDetail
	yamlContent string
	ymlEditor   material.EditorStyle
	ymlArea     widget.Editor
}

func NewYamlDetail(th *material.Theme, item *unstructured.Unstructured) common.IResourceDetail {
	d := &YamlDetail{
		ResourceDetail: NewDetail(th, "yaml", item),
	}
	bytes, err := yaml.Marshal(item)
	if err != nil {
		d.yamlContent = err.Error()
	} else {
		d.yamlContent = string(bytes)
	}

	d.ymlEditor = material.Editor(th, &d.ymlArea, "Yaml")
	d.ymlEditor.Font.Typeface = "monospace"
	d.ymlEditor.Editor.ReadOnly = true
	d.ymlEditor.TextSize = unit.Sp(16)
	d.ymlEditor.LineHeight = unit.Sp(16)

	d.ymlArea.SetText(d.yamlContent)

	return d
}

func (yd *YamlDetail) GetContent() layout.Widget {
	return yd.ymlEditor.Layout
}

func (yd *YamlDetail) Changed() bool {
	return false
}

const POD_LOG_CHANGE_FLAG = "pod.log.detail.log.change.flag"

type PodLogDetail struct {
	*ResourceDetail
	containerLogs []*ContainerLog

	currentLog *ContainerLog
	conLogList layout.List
	logEditor  *common.ReadOnlyEditor
	divider    component.DividerStyle
	theme      *material.Theme

	bufferLimit int //current the ui can't handle large block of text, so we have a limit here
}

// Changed implements IResourceDetail.
func (p *PodLogDetail) Changed() bool {
	for _, cl := range p.containerLogs {
		if cl.Changed() {
			return true
		}
	}
	return false
}

func (cl *ContainerLog) GetDividerWidth(gtx layout.Context, tabFunc func(gtx layout.Context, cl *ContainerLog) layout.Dimensions) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	size := tabFunc(gtx, cl)
	macro.Stop()

	return size
}

// GetContent implements IResourceDetail.
func (p *PodLogDetail) GetContent() layout.Widget {

	containerTab := func(gtx layout.Context, conLog *ContainerLog) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						// the icon
						gtx.Constraints.Min.X = 16
						if conLog.status == nil {
							logger.Error("no status for container!", zap.String("name", conLog.name))
							return layout.Dimensions{}
						} else {
							return conLog.status.Layout(gtx, 16, nil)
						}
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						// the label
						return material.Clickable(gtx, &conLog.clickable, func(gtx layout.Context) layout.Dimensions {
							return conLog.label.Layout(gtx)
						})
					}),
				)
			}),
		)
	}

	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// container tabs
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {

				return p.conLogList.Layout(gtx, len(p.containerLogs), func(gtx layout.Context, index int) layout.Dimensions {
					conLog := p.containerLogs[index]

					if conLog.clickable.Clicked(gtx) || conLog.status.Clickable.Clicked(gtx) {
						if p.currentLog != conLog {
							p.currentLog = conLog
							p.logEditor.SetText(p.currentLog.GetContent(false))
						}
					}
					if p.currentLog == conLog {
						conLog.label.Font.Weight = font.Bold
						conLog.label.Color = common.COLOR.Blue()
						dim := conLog.GetDividerWidth(gtx, containerTab)
						size := dim.Size.X

						return layout.Inset{Top: 0, Bottom: unit.Dp(16), Left: 0, Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									gtx.Constraints.Min.Y = dim.Size.Y
									return containerTab(gtx, conLog)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									gtx.Constraints.Min.X = size
									return conLog.detail.divider.Layout(gtx)
								}),
							)
						})
					}
					conLog.label.Font.Weight = font.Normal
					conLog.label.Color = common.COLOR.Black()
					return layout.Inset{Top: 0, Bottom: unit.Dp(6), Left: 0, Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return containerTab(gtx, conLog)
					})
				})
			}),
			// log content panel showing current container's log
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				if p.currentLog != nil && p.currentLog.Changed() {
					p.logEditor.SetText(p.currentLog.GetContent(false))
				}
				return p.logEditor.Layout(gtx)
			}),
		)
	}
}

type ContainerLog struct {
	detail *PodLogDetail
	// mutex to protect buffer
	// simpler than a channel
	name      string
	clickable widget.Clickable
	label     material.LabelStyle

	mutex       sync.RWMutex
	buffer      *bytes.Buffer
	logReadTask *common.LongTask

	fullLog              *string
	warned               bool
	logChangeContextFlag string

	status *common.StatusIcon
}

func (cl *ContainerLog) addLog(newLog string) {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	cl.buffer.WriteString(newLog)

	common.SetContextBool(cl.logChangeContextFlag, true, nil)
}

func (cl *ContainerLog) resetBuffer() {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	cl.buffer.Reset()
	cl.logReadTask = nil
}

func (cl *ContainerLog) Changed() bool {
	changed, _, _ := common.GetContextBool(cl.logChangeContextFlag)
	if changed {
		common.FlipContextBool(cl.logChangeContextFlag)
	}
	return changed
}

var readingMsg = "Reading..."

func (cl *ContainerLog) LoadLog(reload bool) {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	if cl.logReadTask == nil {
		ctxData, _ := common.GetContextData(common.CONTEXT_LONG_TASK_LIST)
		if taskCtx, ok := ctxData.(*common.LongTasksContext); ok {
			cl.logReadTask = taskCtx.AddTask("Reading pod log")
			cl.logReadTask.Progress = 0.1
			cl.logReadTask.Step = 0.01
			cl.logReadTask.Run = func() {
				client := k8sservice.GetK8sService()
				ioReader, err := client.GetPodLog(cl.detail.item, cl.name)
				if err != nil {
					msg := err.Error()
					cl.addLog(msg) //trigger update
					cl.logReadTask.Done()
					return
				} else {
					defer ioReader.Close()
				}
				if cl.logReadTask.Progress > 0.9 {
					//dont let it reach 100% until done
					cl.logReadTask.Step = 0.0001
				}

				tot := 0
				bts := make([]byte, 2048)

				for {
					n, err := ioReader.Read(bts)
					if err != nil {
						if err == io.EOF {
							cl.addLog("") //trigger update
							cl.logReadTask.Done()
						} else {
							cl.addLog("\n- error occured while reading log -\n")
							cl.addLog(err.Error())
							cl.logReadTask.Failed(err)
						}
						break
					} else if n > 0 {
						cl.addLog(string(bts[:n]))
						cl.logReadTask.Update(fmt.Sprintf("read %d\n", n))
						tot += n
					} else {
						cl.logReadTask.Update(fmt.Sprintf("total %d\n", tot))
						cl.logReadTask.Done()
						cl.addLog("") //trigger update
						break
					}
				}
				logger.Info("total log", zap.String("con", cl.name), zap.Int("total", tot))
			}
			cl.logReadTask.Start()
		}

	} else {
		if cl.logReadTask.IsDone() && reload {
			go func() {
				cl.resetBuffer()
				cl.LoadLog(false)
			}()
		}
	}
}

func (cl *ContainerLog) GetContent(refresh bool) *string {

	cl.LoadLog(refresh)

	return cl.currentLog()
}

func (cl *ContainerLog) currentLog() *string {
	cl.mutex.RLock()
	defer cl.mutex.RUnlock()

	if cl.logReadTask != nil && !cl.logReadTask.IsDone() {
		// reading started but no content read yet
		if cl.buffer.Len() == 0 {
			return &readingMsg
		}
	}

	total := cl.buffer.Len()
	if total >= cl.detail.bufferLimit {
		if !cl.warned {
			logger.Info("the log exceeds max limit and will be truncated", zap.Int("limit", cl.detail.bufferLimit))
			cl.warned = true
		}
		warnMsg := fmt.Sprintf("...(log truncated as is too large: %d)\n", total)
		trLog := warnMsg + cl.buffer.String()[cl.buffer.Len()-cl.detail.bufferLimit:]
		return &trLog
	}

	if cl.logReadTask.IsDone() && cl.fullLog != nil {
		*cl.fullLog = cl.buffer.String()
	}

	return cl.fullLog
}

type ReloadLogAction struct {
	Name     string
	btn      widget.Clickable
	MenuFunc func(gtx layout.Context) layout.Dimensions
	podLog   *PodLogDetail
}

// Execute implements common.MenuAction.
func (r *ReloadLogAction) Execute(gtx layout.Context, se *common.ReadOnlyEditor) error {
	if r.podLog.currentLog != nil {
		r.podLog.currentLog.LoadLog(true)
	}
	return nil
}

// GetClickable implements common.MenuAction.
func (r *ReloadLogAction) GetClickable() *widget.Clickable {
	return &r.btn
}

// GetMenuOption implements common.MenuAction.
func (r *ReloadLogAction) GetMenuOption() func(gtx layout.Context) layout.Dimensions {
	return r.MenuFunc
}

// GetName implements common.MenuAction.
func (r *ReloadLogAction) GetName() string {
	return r.Name
}

func NewReloadLogAction(th *material.Theme, logDetail *PodLogDetail) *ReloadLogAction {
	reloadAct := &ReloadLogAction{
		Name:   "Reload",
		podLog: logDetail,
	}
	reloadAct.MenuFunc = func(gtx layout.Context) layout.Dimensions {
		return common.ItemFunc(th, gtx, &reloadAct.btn, reloadAct.Name, graphics.ReloadIcon)
	}
	return reloadAct
}

func (pd *PodLogDetail) Init(th *material.Theme, status common.ResStatusInfo) {
	client := k8sservice.GetK8sService()
	cons, err := client.GetPodContainers(pd.item)
	if err != nil {
		logger.Warn("Failed to get pod containers", zap.Error(err))
		return
	}

	for i, c := range cons {
		cl := &ContainerLog{
			detail:               pd,
			name:                 c,
			label:                material.H6(th, c),
			buffer:               new(bytes.Buffer),
			fullLog:              new(string),
			logChangeContextFlag: fmt.Sprintf("%s%d", POD_LOG_CHANGE_FLAG, i),
		}
		if podStatus, ok := status.(*common.PodStatusInfo); ok {
			if cstatus, ok := podStatus.ContainersInfo[c]; ok {
				cl.status = cstatus.StatusIcon
			}
		}
		cl.label.TextSize = unit.Sp(14)
		common.RegisterContext(cl.logChangeContextFlag, false, true)
		pd.containerLogs = append(pd.containerLogs, cl)
	}

	actions := make([]common.MenuAction, 0)
	reloadLogAct := NewReloadLogAction(th, pd)
	actions = append(actions, reloadLogAct)

	pd.logEditor = common.NewReadOnlyEditor(th, "log", 16, actions)

	pd.divider = component.Divider(th)
	pd.divider.Fill = common.COLOR.Gray()
	pd.divider.Thickness = unit.Dp(1)
	pd.divider.Inset.Top = 0
	pd.divider.Bottom = unit.Dp(10)

	pd.theme = th
}

func NewPodLogDetail(item *unstructured.Unstructured, th *material.Theme, status common.ResStatusInfo) *PodLogDetail {
	pd := &PodLogDetail{
		ResourceDetail: NewDetail(th, "logs", item),
		containerLogs:  make([]*ContainerLog, 0),
		bufferLimit:    1024 * 1024 * 10,
	}
	pd.Init(th, status)

	return pd
}

func GetExtApiDetails(item *unstructured.Unstructured, th *material.Theme, status common.ResStatusInfo) []common.IResourceDetail {
	result := make([]common.IResourceDetail, 0)

	if item.GetKind() == "Pod" {
		result = append(result, NewPodLogDetail(item, th, status))
	}

	return result
}
