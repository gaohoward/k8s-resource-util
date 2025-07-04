package options

type AppOptions struct {
	Mode       string
	Kubeconfig string
}

var Options = AppOptions{
	Mode:       "gui",
	Kubeconfig: "",
}
