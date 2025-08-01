package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gaohoward.tools/k8s/resutil/pkg/appui"
	"gaohoward.tools/k8s/resutil/pkg/common"
	"gaohoward.tools/k8s/resutil/pkg/config"
	"gaohoward.tools/k8s/resutil/pkg/k8sservice"
	"gaohoward.tools/k8s/resutil/pkg/logs"
	"gaohoward.tools/k8s/resutil/pkg/options"
	"k8s.io/client-go/util/homedir"

	"gioui.org/app"
	"gioui.org/unit"
	"go.uber.org/zap"
)

var logger *zap.Logger

func init() {
	logger, _ = logs.NewAppLogger(logs.MAIN_LOGGER_NAME)
}

func run(window *app.Window, appUi *appui.AppUI) error {
	go func() {
		for {
			select {
			case code := <-appUi.RefreshCh:
				if code == 0 {
					return
				}
				appUi.ForceUpdate = true
				window.Invalidate()
			case <-time.After(60 * time.Second):
				//we can use this to display a timer
			}
		}
	}()
	for {
		e := window.Event()
		common.GetExplorer().ListenEvents(e)
		switch e := e.(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			// This graphics context is used for managing the rendering state.
			gtx := app.NewContext(&appUi.Ops, e)

			// paint.ColorOp{Color: color.NRGBA{R: 0, G: 191, B: 255, A: 255}}.Add(gtx.Ops)
			// paint.PaintOp{}.Add(gtx.Ops)

			// simply move the whole app area within the window to X,Y

			// defer op.Offset(image.Point{X: 50, Y: 0}).Add(&appUi.Ops)

			appUi.Layout(gtx)
			// Pass the drawing operations to the GPU.
			e.Frame(gtx.Ops)
		}
	}
}

func runAgent() {
	k8sservice.Run()
}

// Options
// --kubeconfig <kubecfg local dir> | agent=host:port
// --mode <agent|gui> default gui
// if mode is agent, --kubeconfig must be a local kubeconfig
func main() {
	defer logger.Sync()

	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	var mode *string = flag.String("mode", "gui", "running mode")

	var useCompressor *bool = flag.Bool("grpc-compression", true, "Whether to use compression in grpc")

	flag.Parse()

	options.Options.Mode = *mode
	options.Options.Kubeconfig = *kubeconfig
	options.Options.UseCompressor = *useCompressor

	k8sservice.InitK8sService()

	if *mode == "agent" {
		if strings.HasPrefix(*kubeconfig, "agent=") {
			logger.Info("agent mode doesn't support remote k8s client")
			return
		}
		logger.Info("starting local agent...")
		runAgent()
		return
	}

	cfgDir, err := config.GetConfigDir()
	if err != nil {
		logger.Error("error getting config dir", zap.Error(err))
		return
	}

	logger.Info("parsing", zap.String("configDir", cfgDir))

	appUI := appui.NewAppUI(logger)

	go func() {
		appWin := common.GetAppWindow()
		//appWin.Option(app.Fullscreen.Option())
		appWin.Option(app.Title("K8s Resource Util"))
		appWin.Option(app.Size(unit.Dp(1280), unit.Dp(800)))
		err := run(appWin, appUI)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()

	//consider using sync.Once
	logger.Info("initializing ...")
	errs := appUI.Init()
	if len(errs) > 0 {
		logger.Error("failed to init app", zap.Int("total errs", len(errs)))
		for i := range errs {
			logger.Error("error initializing app", zap.Error(errs[i]))
		}
		os.Exit(1)
	}
	logger.Info("initialization done")

	app.Main()
}
