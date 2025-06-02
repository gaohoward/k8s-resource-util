package panels

import (
	"strings"

	"gaohoward.tools/k8s/resutil/pkg/graphics"
	"gaohoward.tools/k8s/resutil/pkg/logs"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"go.uber.org/zap"
)

type LogBuffer struct {
	buffer  strings.Builder
	changed bool
}

func (l *LogBuffer) ToString() string {
	l.changed = false
	return l.buffer.String()
}

func (l *LogBuffer) Changed() bool {
	return l.changed
}

func (l *LogBuffer) Reset() {
	l.buffer.Reset()
	l.changed = true
}

func (l *LogBuffer) WriteString(s string) {
	l.buffer.WriteString(s)
	l.changed = true
}

type LogTab struct {
	clearBtn        widget.Clickable
	clearBtnTooltip component.Tooltip
	clearBtnTipArea component.TipArea

	logTab       widget.Clickable
	logEditor    material.EditorStyle
	logArea      widget.Editor
	logger       *zap.Logger
	logBuffer    *LogBuffer
	rigidButtons []layout.FlexChild
	widget       layout.Widget
}

// GetClickable implements PanelTab.
func (p *LogTab) GetClickable() *widget.Clickable {
	return &p.logTab
}

// GetTabButtons implements PanelTab.
func (p *LogTab) GetTabButtons(th *material.Theme) []layout.FlexChild {
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

// Write implements io.Writer.
// This has a problem because it causes rendering problem
// if called in a go routine
func (p *LogTab) Write(data []byte) (n int, err error) {
	p.logBuffer.WriteString(string(data))
	return len(data), nil
}

func NewLogTab(th *material.Theme) *LogTab {
	tab := &LogTab{}
	tab.logBuffer = &LogBuffer{}
	var err error
	tab.logger, err = logs.NewAppLogger(logs.IN_APP_LOGGER_NAME, tab)

	if err != nil {
		tab.logger = zap.NewNop()
		tab.logArea.SetText("No log due to logger creation error " + err.Error())
	} else {
		tab.logArea.SetText("panel log")
	}

	tab.logEditor = material.Editor(th, &tab.logArea, "log")
	//tab.logEditor.Font.Typeface = "monospace"
	tab.logEditor.Editor.ReadOnly = true
	tab.logEditor.TextSize = unit.Sp(14)
	tab.logEditor.LineHeight = unit.Sp(14)

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
			tab.logBuffer.Reset()
			tab.logArea.SetText("")
		}
		return layout.Inset{Top: 4, Bottom: 0, Left: 0, Right: 4}.Layout(gtx, clearBtn.Layout)
	})
	tab.rigidButtons = append(tab.rigidButtons, rigid1)

	tab.widget = func(gtx layout.Context) layout.Dimensions {
		//update content only if it has changes
		if tab.logBuffer.Changed() {
			tab.logArea.SetText(tab.logBuffer.ToString())
		}
		return tab.logEditor.Layout(gtx)
	}

	return tab
}
