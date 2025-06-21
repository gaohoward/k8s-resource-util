package appui

import (
	"fmt"
	"image"
	"time"

	"gaohoward.tools/k8s/resutil/pkg/appdrawer"
	"gaohoward.tools/k8s/resutil/pkg/common"
	"gaohoward.tools/k8s/resutil/pkg/dialogs"
	"gaohoward.tools/k8s/resutil/pkg/graphics"
	"gaohoward.tools/k8s/resutil/pkg/logs"
	"gaohoward.tools/k8s/resutil/pkg/panels"
	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"go.uber.org/zap"
)

var logger *zap.Logger

func init() {
	logger, _ = logs.NewAppLogger("appui")
}

type AboutAction struct {
}

// All Init() funcs are called before layout
// All setup* funcs are used to layout sub components
type ControlBar struct {
	bar             *component.AppBar
	overflowAbout   *AboutAction
	globalOverflows []component.OverflowAction
}

func (cbar *ControlBar) SetActions(actions []component.AppBarAction) {
	cbar.bar.SetActions(actions, cbar.globalOverflows)
}

func (cbar *ControlBar) init(m *component.ModalLayer, th *material.Theme) {
	cbar.bar = component.NewAppBar(m)
	cbar.bar.NavigationIcon = graphics.MenuIcon

	cbar.globalOverflows = []component.OverflowAction{
		{
			Name: "About",
			Tag:  cbar.overflowAbout,
		},
	}
	actions, _ := graphics.GetActions(th)
	cbar.SetActions(actions)
}

type ResourceDrawer struct {
	nav     component.NavDrawer
	drawer  *component.ModalNavDrawer
	navAnim *component.VisibilityAnimation
}

func (resDrawer *ResourceDrawer) init(m *component.ModalLayer) {
	resDrawer.nav = component.NewNav("Quick List", "Generate samples for a type")
	resDrawer.drawer = component.ModalNavFrom(&resDrawer.nav, m)
	resDrawer.navAnim = &component.VisibilityAnimation{
		State:    component.Invisible,
		Duration: time.Millisecond * 250,
	}
	for _, rp := range appdrawer.Items {
		navItem := rp.GetNaviItem()
		navItem.Tag = rp.GetKey()
		resDrawer.drawer.AddNavItem(navItem)
	}
	resDrawer.drawer.UnselectNavDestination()
}

type ResourceNavigator struct {
	modal          *component.ModalLayer
	constrolBar    ControlBar
	resourceDrawer ResourceDrawer
	floatMenu      bool
}

func (navigator *ResourceNavigator) setup(gtx layout.Context, th *material.Theme) *layout.FlexChild {

	for _, event := range navigator.constrolBar.bar.Events(gtx) {
		switch event := event.(type) {
		case component.AppBarNavigationClicked:
			if !navigator.floatMenu {
				navigator.resourceDrawer.navAnim.ToggleVisibility(gtx.Now)
			} else {
				navigator.resourceDrawer.drawer.Appear(gtx.Now)
				navigator.resourceDrawer.navAnim.Disappear(gtx.Now)
			}
		case component.AppBarContextMenuDismissed:
			logger.Info("Context menu dismissed", zap.Any("e", event))
		case component.AppBarOverflowActionClicked:
			if aboutAction, ok := event.Tag.(*AboutAction); ok {
				common.SetContextBool(common.CONTEXT_SHOW_ABOUT, true, aboutAction)
			}
		}
	}

	child := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return navigator.constrolBar.bar.Layout(gtx, th, "Menu", "Actions")
	})

	return &child

}

func (navigator *ResourceNavigator) init(th *material.Theme) {
	navigator.modal = component.NewModal()
	// appbar and drawer share the modal
	navigator.constrolBar.init(navigator.modal, th)
	navigator.resourceDrawer.init(navigator.modal)
	navigator.floatMenu = true

	navigator.constrolBar.overflowAbout = &AboutAction{}
}

type AppUI struct {
	Logger            *zap.Logger
	theme             *material.Theme
	Ops               op.Ops
	resourceNavigator ResourceNavigator
	resourcePage      ResourcePage

	panel       *panels.AppPanel
	panelResize component.Resize

	info                string
	progressBar         material.ProgressBarStyle
	RefreshCh           chan int
	ForceUpdate         bool
	resize1             component.Resize
	resourceCollections *ResourceCollections
	aboutPanel          *AboutPanel
}

const (
	APP_TITLE = "K8s Resource Utility"
	APP_INTRO = `A tool that can help manage custom resources.

  * Create custom resources
  * Custom resources are saved in a local repo
  * Custom resources are arranged into collections
  * Deploy custom resources or collections into a k8s cluster
  * Manage deployments
  * Provide schemas for api resources to assist CR creation
  * Search api resources in a live cluster

  -- Powered by Gio https://github.com/gioui/gio`
)

type AboutPanel struct {
}

func GetAboutWidth(gtx layout.Context, th *material.Theme) layout.Dimensions {

	macro := op.Record(gtx.Ops)
	label := material.Body1(th, APP_INTRO)
	label.TextSize = unit.Sp(16)
	label.Font.Weight = font.Bold
	size := label.Layout(gtx)
	macro.Stop()

	return size

}

func (a *AboutPanel) Layout(gtx layout.Context, th *material.Theme, isForInit bool) layout.Dimensions {

	label := material.Body1(th, APP_INTRO)
	size := GetAboutWidth(gtx, th)

	childrenSize := 3
	if isForInit {
		childrenSize = 4
	}
	children := make([]layout.FlexChild, childrenSize)

	children[0] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		//title
		titleLb := material.Label(th, unit.Sp(24), APP_TITLE)
		titleLb.Font.Weight = font.Bold
		titleLb.Color = common.COLOR.Blue()

		versionLb := material.Label(th, unit.Sp(14), VERSION)
		versionLb.Font.Weight = font.Bold

		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
			layout.Rigid(titleLb.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 0, Bottom: 0, Left: 4, Right: 0}.Layout(gtx,
					versionLb.Layout)
			},
			),
		)
	})

	children[1] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		if gtx.Constraints.Min.X == 0 {
			gtx.Constraints.Min.X = size.Size.X
		}
		div := component.Divider(th)
		div.Fill = th.Palette.ContrastBg
		div.Top = unit.Dp(4)
		div.Bottom = unit.Dp(4)

		return div.Layout(gtx)
	})

	children[2] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return label.Layout(gtx)
	})

	if isForInit {
		children[3] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			p, _ := common.GetContextData(common.CONTEXT_APP_INIT_STATE)

			progress := float32(0.0)
			if prg, ok := p.(float32); ok {
				progress = prg
			}

			progressBar := material.ProgressBar(th, progress)
			progressBar.Height = unit.Dp(6)
			gtx.Constraints.Max.X = GetAboutWidth(gtx, th).Size.X
			if progressBar.Progress == 0.0 {
				progressBar.Color = th.ContrastFg
			} else {
				progressBar.Color = common.COLOR.Blue()
			}
			return progressBar.Layout(gtx)
		})
	}

	return layout.UniformInset(unit.Dp(20)).Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				children...,
			)
		})
}

func (appUi *AppUI) Init() []error {

	common.SetContextData(common.CONTEXT_APP_INIT_STATE, float32(0.0), nil)

	appUi.RefreshCh = make(chan int, 1)
	appUi.ForceUpdate = false
	appUi.theme.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	appUi.resourceNavigator.init(appUi.theme)

	var err []error
	appUi.resourceCollections, err = GetResourceCollections(&appUi.resourcePage, appUi.theme)
	if len(err) > 0 {
		return err
	}

	common.SetContextData(common.CONTEXT_APP_INIT_STATE, float32(0.6), nil)

	k8sClient := common.GetK8sClient()

	appUi.resourcePage.Init(k8sClient.GetResUtil(), k8sClient, appUi.RefreshCh, appUi.theme)

	common.SetContextData(common.CONTEXT_APP_INIT_STATE, float32(0.9), nil)

	appUi.panel = appUi.resourcePage.GetPanel()
	appUi.panelResize.Axis = layout.Vertical
	appUi.panelResize.Ratio = 0.6

	appUi.info = "app is running."
	appUi.resize1.Ratio = 0.25

	appUi.progressBar = material.ProgressBar(appUi.theme, 0.0)
	appUi.progressBar.Height = unit.Dp(6)

	appUi.aboutPanel = &AboutPanel{}

	common.SetContextData(common.CONTEXT_APP_INIT_STATE, float32(1.0), nil)
	return nil
}

func (appUi *AppUI) layoutResourceArea(gtx layout.Context) layout.Dimensions {
	return appUi.resourcePage.Layout(gtx, appUi.theme)
}

func (appUi *AppUI) layoutPanelArea(gtx layout.Context) layout.Dimensions {
	return appUi.panel.GetWidget()(gtx)
}

func (appUi *AppUI) setupAppBar(gtx layout.Context, th *material.Theme) *layout.FlexChild {

	if common.GetK8sClient().IsValid() {
		appUi.resourceNavigator.constrolBar.bar.Title = common.GetK8sClient().GetClusterInfo().Host
	} else {
		appUi.resourceNavigator.constrolBar.bar.Title = "no cluster connected"
	}

	return appUi.resourceNavigator.setup(gtx, th)

}

func (appUi *AppUI) setupCollections() *layout.Widget {
	var collections layout.Widget = func(gtx layout.Context) layout.Dimensions {
		return appUi.resourceCollections.Layout(gtx, appUi.theme)
	}
	return &collections
}

func (appUi *AppUI) setupMainContent() *layout.Widget {
	var content layout.Widget = func(gtx layout.Context) layout.Dimensions {
		showPanel := graphics.ShowPanel()
		if showPanel {
			return appUi.panelResize.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return appUi.layoutResourceArea(gtx)
				},
				func(gtx layout.Context) layout.Dimensions {
					return appUi.layoutPanelArea(gtx)
				},
				common.HorizontalSplitHandler,
			)
		}
		return appUi.layoutResourceArea(gtx)
	}
	return &content
}

func (appUi *AppUI) setupBottomBar() *layout.FlexChild {
	bottom := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			appUi.getBottomBarComponents()...,
		)
	})
	return &bottom
}

func (appUi *AppUI) getBottomBarComponents() []layout.FlexChild {
	flexChildren := make([]layout.FlexChild, 0)
	flexChildren = append(flexChildren,
		layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
			return material.Label(appUi.theme, unit.Sp(16), appUi.info).Layout(gtx)
		}))
	ctxData, _ := common.GetContextData(common.CONTEXT_LONG_TASK_LIST)
	var firstTask *common.LongTask = nil
	if ld, ok := ctxData.(*common.LongTasksContext); ok {
		appUi.info = "App is running"
		firstTask = ld.GetFirstTask()
	}
	if firstTask != nil {
		appUi.info = fmt.Sprintf("Running Task: %v", firstTask.Name)
		// The progress bar
		flexChildren = append(flexChildren,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				//progress bar to the right
				gtx.Constraints.Max.X = 200

				appUi.progressBar.Progress = firstTask.Progress

				if appUi.progressBar.Progress == 0.0 {
					appUi.progressBar.Color = appUi.theme.ContrastFg
				} else {
					appUi.progressBar.Color = common.COLOR.Blue()
				}
				return appUi.progressBar.Layout(gtx)
			}),
		)
	}

	return flexChildren
}

var mousePosition f32.Point

func (appUi *AppUI) Layout(gtx layout.Context) {

	if !appUi.InitDone() {

		layout.Stack{Alignment: layout.Center}.Layout(gtx,
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{
						Size: image.Point{X: gtx.Constraints.Max.X, Y: gtx.Constraints.Max.Y},
					}
				})
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return component.Surface(appUi.theme).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return appUi.aboutPanel.Layout(gtx, appUi.theme, true)
				})
			}),
		)
		return
	}

	current := appUi.resourceNavigator.resourceDrawer.drawer.CurrentNavDestination()
	if appUi.resourceNavigator.resourceDrawer.drawer.NavDestinationChanged() {
		if current != nil {
			tag := current.(common.BuiltinKind)
			appUi.resourcePage.OpenTemplate(tag)
		}
		appUi.resourceNavigator.resourceDrawer.navAnim.ToggleVisibility(gtx.Now)
	}
	if appUi.ForceUpdate {
		appUi.ForceUpdate = false
		if current != nil {
			appUi.resourcePage.UpdateCRD()
		}
	}

	flex := layout.Flex{Axis: layout.Vertical}
	bar := appUi.setupAppBar(gtx, appUi.theme)

	cols := appUi.setupCollections()
	content := appUi.setupMainContent()

	// sample to create a scrollbar
	// many of the gio widgets don't have native scrollbar support
	// e.g. the Editor is scrollable but no scroll bar
	// if you need a scrollbar you need to handle everything
	// sb := widget.Scrollbar{}
	// scrollBar := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
	// 	sbstyle := material.Scrollbar(appUi.theme, &sb)
	// 	dim := sbstyle.Layout(gtx, layout.Vertical, 0.0, 0.1)
	// 	return dim
	// })
	// reig := layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
	// 	return layout.Flex{}.Layout(gtx, *drawer, *content, scrollBar)
	// })

	reig := layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {

		return appUi.resize1.Layout(gtx, *cols, *content,
			common.VerticalSplitHandler,
		)
	})

	bottomBar := appUi.setupBottomBar()

	if appUi.isShowAboutDialog() {

		r := image.Rectangle{Max: image.Point{X: gtx.Constraints.Max.X, Y: gtx.Constraints.Max.Y}}

		area := clip.Rect(r).Push(gtx.Ops)

		for {
			_, ok := gtx.Event(pointer.Filter{
				Target: &mousePosition,
				Kinds:  pointer.Press,
			}, key.Filter{Name: ""})
			if !ok {
				break
			}
			appUi.closeAboutDialog()
		}

		event.Op(gtx.Ops, &mousePosition)
		defer area.Pop()

		layout.Stack{Alignment: layout.Center}.Layout(gtx,
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return flex.Layout(gtx, *bar, reig, *bottomBar)
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return component.Surface(appUi.theme).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return appUi.aboutPanel.Layout(gtx, appUi.theme, false)
				})
			}),
		)
	} else {
		flex.Layout(gtx, *bar, reig, *bottomBar)
	}

	appUi.resourceNavigator.modal.Layout(gtx, appUi.theme)

	dialogs.LayoutDialogs(gtx, appUi.theme)
}

func (appUi *AppUI) InitDone() bool {
	done, _ := common.GetContextData(common.CONTEXT_APP_INIT_STATE)
	if prg, ok := done.(float32); ok {
		return prg >= float32(1.0)
	}
	return false
}

func (appUi *AppUI) closeAboutDialog() {
	common.SetContextBool(common.CONTEXT_SHOW_ABOUT, false, nil)
}

func (appUi *AppUI) isShowAboutDialog() bool {
	show, _, err := common.GetContextBool(common.CONTEXT_SHOW_ABOUT)
	if err != nil {
		logger.Info("failed to get context", zap.Error(err))
		return false
	}
	return show
}

func NewAppUI(logger *zap.Logger) *AppUI {

	common.RegisterContext(common.CONTEXT_APP_INIT_STATE, float32(0.0), true)
	common.RegisterContext(common.CONTEXT_SHOW_ABOUT, false, true)

	appUI := &AppUI{
		Logger:       logger,
		theme:        material.NewTheme(),
		resourcePage: ResourcePage{},
	}

	//	appUI.theme.Bg = color.NRGBA{0, 128, 0, 50}
	// appUI.theme.Face = "monospace"
	return appUI
}
