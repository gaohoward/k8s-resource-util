package panels

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"

	"crypto/x509"

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
	"github.com/smallstep/certinfo"
	om "github.com/wk8/go-ordered-map/v2"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type ResourceDetail struct {
	detailName string
	clickable  widget.Clickable
	label      material.LabelStyle
	item       *unstructured.Unstructured
	isSelected bool
}

func NewDetail(name string, item *unstructured.Unstructured) *ResourceDetail {
	d := &ResourceDetail{
		detailName: name,
		label:      material.H6(common.GetTheme(), name),
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
		r.label.Color = common.COLOR.Blue
		r.label.Font.Weight = font.Bold
	} else {
		r.label.Color = common.COLOR.Black
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

// Save implements common.IResourceDetail.
func (dd *DescribeDetail) Save(baseDir string, kind string, name string, ns string) {
	filePath := common.CreateFilePathForK8sObject(baseDir, kind, name, ns, "describe", "txt")
	content := dd.contentEditor.GetText()
	err := common.SaveFile(filePath, &content)

	if err != nil {
		logger.Error("Failed to save describe content", zap.String("file", filePath), zap.Error(err))
	}
	logger.Info("successfully saved described file", zap.String("path", filePath))
}

func NewDescribeDetail(item *unstructured.Unstructured) common.IResourceDetail {
	d := &DescribeDetail{
		ResourceDetail: NewDetail("describe", item),
	}

	d.contentEditor = common.NewReadOnlyEditor("describe", 16, nil, nil, true)

	d.contentEditor.SetText(d.getDescribeContent(), nil)

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
	ymlEditor   *common.ReadOnlyEditor
}

// Save implements common.IResourceDetail.
func (yd *YamlDetail) Save(baseDir string, kind string, name string, ns string) {
	filePath := common.CreateFilePathForK8sObject(baseDir, kind, name, ns, "yaml", "yaml")
	content := yd.ymlEditor.GetText()
	err := common.SaveFile(filePath, &content)

	if err != nil {
		logger.Error("Failed to save yaml content", zap.String("file", filePath), zap.Error(err))
	}
	logger.Info("successfully saved yaml file", zap.String("path", filePath))
}

func NewYamlDetail(item *unstructured.Unstructured) common.IResourceDetail {
	d := &YamlDetail{
		ResourceDetail: NewDetail("yaml", item),
	}

	yamlStr, err := common.MarshalYaml(item)

	if err != nil {
		d.yamlContent = err.Error()
	} else {
		d.yamlContent = yamlStr
	}

	d.ymlEditor = common.NewReadOnlyEditor("Yaml", 16, nil, nil, true)
	d.ymlEditor.SetText(&d.yamlContent, nil)

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

	bufferLimit int //current the ui can't handle large block of text, so we have a limit here
}

// Save implements common.IResourceDetail.
func (p *PodLogDetail) Save(baseDir string, kind string, name string, ns string) {
	for _, conLog := range p.containerLogs {

		cat := conLog.name + "-log"

		filePath := common.CreateFilePathForK8sObject(baseDir, kind, name, ns, cat, "log")
		err := common.SaveFile(filePath, conLog.GetContent(false))

		if err != nil {
			logger.Error("Failed to save container log", zap.String("file", filePath), zap.Error(err))
		}
		logger.Info("successfully saved container log", zap.String("path", filePath))
	}
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
							p.logEditor.SetText(p.currentLog.GetContent(false), nil)
						}
					}
					if p.currentLog == conLog {
						conLog.label.Font.Weight = font.Bold
						conLog.label.Color = common.COLOR.Blue
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
					conLog.label.Color = common.COLOR.Black
					return layout.Inset{Top: 0, Bottom: unit.Dp(6), Left: 0, Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return containerTab(gtx, conLog)
					})
				})
			}),
			// log content panel showing current container's log
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				if p.currentLog != nil && p.currentLog.Changed() {
					p.logEditor.SetText(p.currentLog.GetContent(false), nil)
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
func (r *ReloadLogAction) Execute(gtx layout.Context, se *common.ReadOnlyEditor, _ *common.Liner) error {
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

func NewReloadLogAction(logDetail *PodLogDetail) *ReloadLogAction {
	reloadAct := &ReloadLogAction{
		Name:   "Reload",
		podLog: logDetail,
	}
	reloadAct.MenuFunc = func(gtx layout.Context) layout.Dimensions {
		return common.ItemFunc(gtx, &reloadAct.btn, reloadAct.Name, graphics.ReloadIcon)
	}
	return reloadAct
}

func (pd *PodLogDetail) Init(status common.ResStatusInfo) error {
	th := common.GetTheme()
	cons, err := common.GetPodContainers(pd.item)
	if err != nil {
		logger.Warn("Failed to get pod containers", zap.Error(err))
		return err
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
	reloadLogAct := NewReloadLogAction(pd)
	actions = append(actions, reloadLogAct)

	pd.logEditor = common.NewReadOnlyEditor("log", 15, actions, nil, true)

	pd.divider = component.Divider(th)
	pd.divider.Fill = common.COLOR.Gray
	pd.divider.Thickness = unit.Dp(1)
	pd.divider.Inset.Top = 0
	pd.divider.Bottom = unit.Dp(10)

	return nil
}

func NewPodLogDetail(item *unstructured.Unstructured, status common.ResStatusInfo) (*PodLogDetail, error) {
	pd := &PodLogDetail{
		ResourceDetail: NewDetail("logs", item),
		containerLogs:  make([]*ContainerLog, 0),
		bufferLimit:    1024 * 1024 * 10,
	}
	err := pd.Init(status)
	if err != nil {
		return nil, err
	}

	return pd, nil
}

func GetExtApiDetails(item *unstructured.Unstructured, status common.ResStatusInfo) []common.IResourceDetail {
	result := make([]common.IResourceDetail, 0)

	if item.GetKind() == "Pod" {
		podDetail, err := NewPodLogDetail(item, status)
		if err != nil {
			logger.Warn("Failed to create pod log detail", zap.Error(err))
		} else {
			result = append(result, podDetail)
		}
	}

	if item.GetKind() == "Secret" {
		secret := &corev1.Secret{}
		if runtime.DefaultUnstructuredConverter == nil {
			logger.Debug("no default converter")
		}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, secret)
		if err != nil {
			logger.Warn("failed to conver unstructured to secret", zap.Any("err", err))
		}

		if secret.Type == corev1.SecretTypeTLS {
			result = append(result, NewSecretTlsDetail(secret, item))
		}

		if len(secret.Data) > 0 {
			result = append(result, NewSecretDataDetail(secret, item))
		}
	}

	if item.GetKind() == "ConfigMap" {
		configMap := &corev1.ConfigMap{}
		if runtime.DefaultUnstructuredConverter == nil {
			logger.Debug("no default converter")
		}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, configMap)
		if err != nil {
			logger.Warn("failed to conver unstructured to configmap", zap.Any("err", err))
		}

		if configMap.Data != nil {
			for _, v := range configMap.Data {
				if strings.HasPrefix(v, "-----BEGIN CERTIFICATE-----") {
					result = append(result, NewConfigMapCertDetail(configMap, item))
					break
				}
			}
		}
	}

	return result
}

func NewConfigMapCertDetail(cm *corev1.ConfigMap, item *unstructured.Unstructured) *ConfigMapCertDetail {
	cmcd := &ConfigMapCertDetail{
		ResourceDetail: NewDetail("cert", item),
		configMap:      cm,
	}

	cmcd.editor = common.NewReadOnlyEditor("cert", 15, nil, nil, true)

	cmcd.editor.SetText(cmcd.getCertContent(), nil)

	return cmcd
}

type ConfigMapCertDetail struct {
	*ResourceDetail
	configMap *corev1.ConfigMap
	editor    *common.ReadOnlyEditor
}

// Changed implements common.IResourceDetail.
func (cmcd *ConfigMapCertDetail) Changed() bool {
	return false
}

// GetContent implements common.IResourceDetail.
func (cmcd *ConfigMapCertDetail) GetContent() layout.Widget {
	return cmcd.editor.Layout
}

// Save implements common.IResourceDetail.
func (cmcd *ConfigMapCertDetail) Save(baseDir string, kind string, name string, ns string) {
}

func (cmcd *ConfigMapCertDetail) getCertContent() *string {
	certMap := om.New[string, []*x509.Certificate]()
	for k, v := range cmcd.configMap.Data {
		if strings.HasPrefix(v, "-----BEGIN CERTIFICATE-----") {
			//possible candidate
			certList, err := common.ParseCerts([]byte(v))
			if err != nil {
				continue
			}
			certMap.Set(k, certList)
		}
	}
	builder := strings.Builder{}
	for pair := certMap.Oldest(); pair != nil; pair = pair.Next() {
		builder.WriteString(pair.Key)
		builder.WriteString(":\n")
		total := len(pair.Value)
		for i, c := range pair.Value {
			certText, err := certinfo.CertificateText(c)
			if err != nil {
				builder.WriteString(fmt.Sprintf("- [%d/%d] Failed to get certificate text: %v\n", i+1, total, err))
			} else {
				builder.WriteString(fmt.Sprintf("- [%d/%d] %s\n", i+1, total, certText))
			}
		}
	}
	result := builder.String()
	return &result
}

func NewSecretDataDetail(secret *corev1.Secret, item *unstructured.Unstructured) *SecretDataDetail {
	sdd := &SecretDataDetail{
		ResourceDetail: NewDetail("data", item),
		Secret:         secret,
	}

	sdd.editor = common.NewReadOnlyEditor("data", 15, nil, nil, true)

	sdd.editor.SetText(sdd.getDecodedData(), nil)

	return sdd
}

type SecretDataDetail struct {
	*ResourceDetail
	Secret *corev1.Secret
	editor *common.ReadOnlyEditor
}

// Changed implements common.IResourceDetail.
func (sdd *SecretDataDetail) Changed() bool {
	return false
}

func (sdd *SecretDataDetail) GetContent() layout.Widget {
	return sdd.editor.Layout
}

// Save implements common.IResourceDetail.
func (sdd *SecretDataDetail) Save(baseDir string, kind string, name string, ns string) {
}

func (sdd *SecretDataDetail) getDecodedData() *string {
	builder := strings.Builder{}
	for k, v := range sdd.Secret.Data {
		builder.WriteString(fmt.Sprintf("%s:\n%s\n", k, string(v)))
	}
	result := builder.String()
	return &result
}

func NewSecretTlsDetail(secret *corev1.Secret, item *unstructured.Unstructured) *SecretTlsDetail {
	sd := &SecretTlsDetail{
		ResourceDetail: NewDetail("cert", item),
		Secret:         secret,
		content:        "",
	}

	sd.contentEditor = common.NewReadOnlyEditor("describe", 15, nil, nil, true)

	sd.contentEditor.SetText(sd.getCertContent(), nil)

	return sd
}

type SecretTlsDetail struct {
	*ResourceDetail
	Secret        *corev1.Secret
	contentEditor *common.ReadOnlyEditor
	content       string
}

func (s *SecretTlsDetail) getCertContent() *string {
	if s.content == "" {
		if cert, ok := s.Secret.Data["tls.crt"]; ok {
			certList, err := common.ParseCerts(cert)
			if err != nil {
				s.content = fmt.Sprintf("Failed to parse certificate: %v\n", err)
			}
			totalCerts := len(certList)
			var result string
			for i, cert := range certList {
				certText, err := certinfo.CertificateText(cert)
				if err != nil {
					result += fmt.Sprintf("Failed to get certificate text: %v\n", err)
				} else {
					result += fmt.Sprintf("-[%d/%d]-\n", i+1, totalCerts)
					result += certText + "\n"
				}
			}
			s.content += result
		} else {
			s.content = "No cert data"
		}
	}
	return &s.content
}

func (s *SecretTlsDetail) Changed() bool {
	return false
}

// GetContent implements common.IResourceDetail.
func (s *SecretTlsDetail) GetContent() layout.Widget {
	return s.contentEditor.Layout
}

// Save implements common.IResourceDetail.
func (s *SecretTlsDetail) Save(baseDir string, kind string, name string, ns string) {
}
