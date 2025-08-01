package options

type AppOptions struct {
	Mode          string
	Kubeconfig    string
	UseCompressor bool
}

var Options = AppOptions{
	Mode:          "gui",
	Kubeconfig:    "",
	UseCompressor: true,
}
