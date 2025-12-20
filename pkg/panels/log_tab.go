package panels

import (
	"fmt"
	"strings"

	"gaohoward.tools/k8s/resutil/pkg/common"
	"gaohoward.tools/k8s/resutil/pkg/graphics"
	"gaohoward.tools/k8s/resutil/pkg/logs"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type LogBuffer struct {
	buffer  strings.Builder
	content string
}

func (l *LogBuffer) ToString() string {
	return l.buffer.String()
}

func (l *LogBuffer) Reset() {
	l.buffer.Reset()
}

func (l *LogBuffer) WriteString(s string) {
	l.buffer.WriteString(s)
	l.content = l.buffer.String()
}

type LogTab struct {
	clearBtn        widget.Clickable
	clearBtnTooltip component.Tooltip
	clearBtnTipArea component.TipArea

	logTab       widget.Clickable
	logEditor    *common.ReadOnlyEditor
	logBuffer    *LogBuffer
	logger       *zap.Logger
	rigidButtons []layout.FlexChild
	widget       layout.Widget
}

// GetClickable implements PanelTab.
func (p *LogTab) GetClickable() *widget.Clickable {
	return &p.logTab
}

// GetTabButtons implements PanelTab.
func (p *LogTab) GetTabButtons() []layout.FlexChild {
	return p.rigidButtons
}

// GetTitle implements PanelTab.
func (p *LogTab) GetTitle() string {
	return "log"
}

// GetWidget implements PanelTab.
func (p *LogTab) GetWidget() layout.Widget {
	return p.widget
}

func (p *LogTab) Write(data []byte) (n int, err error) {
	if data != nil {
		logmap := make(map[string]string, 0)
		if err := yaml.Unmarshal(data, logmap); err != nil {
			logger.Error("error unmarshal message", zap.Error(err))
		}

		tm := logmap["ts"]
		msg := logmap["msg"]
		cont, hasContent := logmap[logs.REPLY_CONTENT_KEY]

		builder := strings.Builder{}

		builder.WriteString(tm)
		builder.WriteString(" ")
		builder.WriteString(msg)

		if hasContent {
			content := strings.ReplaceAll(cont, "\\n", "\n")
			content = strings.ReplaceAll(content, "\\\"", "\"")
			builder.WriteString(common.MakeExtraFragment(content))
		}

		builder.WriteString("\n")

		p.logBuffer.WriteString(builder.String())

		p.logEditor.SetText(&p.logBuffer.content, nil)
	}
	return len(data), nil
}

func NewLogTab() *LogTab {
	th := common.GetTheme()

	tab := &LogTab{}
	tab.logEditor = common.NewReadOnlyEditor("log", 14, nil, true)

	tab.logBuffer = &LogBuffer{}

	var err error
	tab.logger, err = logs.NewAppLogger(logs.IN_APP_LOGGER_NAME, tab)

	if err != nil {
		text := fmt.Sprintf("No log due to logger creation error %v\n", err)
		tab.logEditor.SetText(&text, nil)
	}

	tab.rigidButtons = make([]layout.FlexChild, 0)

	tab.clearBtnTooltip = component.DesktopTooltip(th, "Clear")

	clearBtn := component.TipIconButtonStyle{
		Tooltip:         tab.clearBtnTooltip,
		IconButtonStyle: material.IconButton(th, &tab.clearBtn, graphics.ClearIcon, "Clear log"),
		State:           &tab.clearBtnTipArea,
	}

	clearBtn.Size = 16
	clearBtn.IconButtonStyle.Inset = layout.Inset{Top: 1, Bottom: 1, Left: 1, Right: 1}

	rigid1 := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		if tab.clearBtn.Clicked(gtx) {
			tab.logEditor.Clear()
			tab.logBuffer.Reset()
		}
		return layout.Inset{Top: 4, Bottom: 0, Left: 0, Right: 4}.Layout(gtx, clearBtn.Layout)
	})
	tab.rigidButtons = append(tab.rigidButtons, rigid1)

	tab.widget = func(gtx layout.Context) layout.Dimensions {
		return tab.logEditor.Layout(gtx)
	}

	return tab
}
