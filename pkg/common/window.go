package common

import (
	"gioui.org/app"
	"gioui.org/x/explorer"
)

const (
	CONTEXT_SHOW_ABOUT     = "app.show.about"
	CONTEXT_APP_INIT_STATE = "app.init.state"
)

const APP_NAME = "k8sutil"

var appWin *app.Window
var fileChooser *explorer.Explorer

func GetAppWindow() *app.Window {
	if appWin == nil {
		appWin = new(app.Window)
		fileChooser = explorer.NewExplorer(appWin)
	}
	return appWin
}

func GetExplorer() *explorer.Explorer {
	return fileChooser
}
