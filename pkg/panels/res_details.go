package panels

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"gaohoward.tools/k8s/resutil/pkg/common"
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

type IResourceDetail interface {
	GetContent() layout.Widget
	getClickable() *widget.Clickable
	getLabel() layout.Widget
	Changed() bool
	SetSelected(state bool)
}

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

// getClickable implements ResourceDetail.
func (r *ResourceDetail) getClickable() *widget.Clickable {
	return &r.clickable
}

// getWidget implements ResourceDetail.
func (r *ResourceDetail) getLabel() layout.Widget {
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

type YamlDetail struct {
	*ResourceDetail
	yamlContent string
	ymlEditor   material.EditorStyle
	ymlArea     widget.Editor
}

func NewYamlDetail(th *material.Theme, item *unstructured.Unstructured) IResourceDetail {
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

func (cl *ContainerLog) GetDividerWidth(gtx layout.Context) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	size := cl.label.Layout(gtx)
	macro.Stop()

	return size
}

// GetContent implements IResourceDetail.
func (p *PodLogDetail) GetContent() layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// container tabs
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.conLogList.Layout(gtx, len(p.containerLogs), func(gtx layout.Context, index int) layout.Dimensions {
					conLog := p.containerLogs[index]
					if conLog.clickable.Clicked(gtx) {
						if p.currentLog != conLog {
							p.currentLog = conLog
							p.logEditor.SetText(p.currentLog.GetContent(false))
						}
					}
					if p.currentLog == conLog {
						conLog.label.Font.Weight = font.Bold
						conLog.label.Color = common.COLOR.Blue()
						size := conLog.GetDividerWidth(gtx).Size.X
						return layout.Inset{Top: 0, Bottom: unit.Dp(16), Left: 0, Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return material.Clickable(gtx, &conLog.clickable, func(gtx layout.Context) layout.Dimensions {
										return conLog.label.Layout(gtx)
									})
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
						return material.Clickable(gtx, &conLog.clickable, func(gtx layout.Context) layout.Dimensions {
							return conLog.label.Layout(gtx)
						})
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

func (cl *ContainerLog) GetContent(refresh bool) *string {
	if cl.logReadTask == nil {

		ctxData, _ := common.GetContextData(common.CONTEXT_LONG_TASK_LIST)
		if taskCtx, ok := ctxData.(*common.LongTasksContext); ok {
			cl.logReadTask = taskCtx.AddTask("Reading pod log")
			cl.logReadTask.Progress = 0.1
			cl.logReadTask.Step = 0.01
			cl.logReadTask.Run = func() {
				client := common.GetK8sClient()
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
				bts := make([]byte, 2048)
				tot := 0
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
	}
	if cl.logReadTask.IsDone() && refresh {
		cl.resetBuffer()
		cl.GetContent(false)
	}

	if cl.buffer.Len() == 0 && !cl.logReadTask.IsDone() {
		return &readingMsg
	}

	return cl.currentLog()
}

func (cl *ContainerLog) currentLog() *string {
	cl.mutex.RLock()
	defer cl.mutex.RUnlock()

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

func (pd *PodLogDetail) Init(th *material.Theme) {
	client := common.GetK8sClient()
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
		cl.label.TextSize = unit.Sp(14)
		common.RegisterContext(cl.logChangeContextFlag, false, true)
		pd.containerLogs = append(pd.containerLogs, cl)
	}

	pd.logEditor = common.NewReadOnlyEditor(th, "log", 16)

	pd.divider = component.Divider(th)
	pd.divider.Fill = common.COLOR.Gray()
	pd.divider.Thickness = unit.Dp(1)
	pd.divider.Inset.Top = 0
	pd.divider.Bottom = unit.Dp(10)

	pd.theme = th
}

func NewPodLogDetail(item *unstructured.Unstructured, th *material.Theme) *PodLogDetail {
	pd := &PodLogDetail{
		ResourceDetail: NewDetail(th, "logs", item),
		containerLogs:  make([]*ContainerLog, 0),
		bufferLimit:    1024 * 1024 * 10,
	}
	pd.Init(th)

	return pd
}

func GetExtApiDetails(item *unstructured.Unstructured, th *material.Theme) []IResourceDetail {
	result := make([]IResourceDetail, 0)

	if item.GetKind() == "Pod" {
		result = append(result, NewPodLogDetail(item, th))
	}

	return result
}
