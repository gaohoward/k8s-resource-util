package common

import (
	"os"

	"gioui.org/app"
	"gioui.org/x/explorer"
	"go.uber.org/zap"
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

type FileHandler interface {
	FileChoosed(fileUrl string, attachment any) error
	GetFilter() []string // .txt etc
}

func AsyncChooseFile(handler FileHandler, attachment any) {
	explorer := GetExplorer()
	reader, err := explorer.ChooseFile(handler.GetFilter()...)

	if err != nil {
		return
	}
	if reader == nil {
		return
	}
	defer reader.Close()

	if file, ok := reader.(*os.File); ok {
		if err := handler.FileChoosed(file.Name(), attachment); err != nil {
			logger.Warn("error from handler", zap.Error(err))
		}
	} else {
		logger.Info("cannot get file name")
		return
	}

}
