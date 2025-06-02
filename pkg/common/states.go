package common

type DeployState int

const (
	StateNew DeployState = iota
	StateInDeploy
	StateDeployed
)

var stateName = map[DeployState]string{
	StateNew:      "New",
	StateInDeploy: "InDeploy",
	StateDeployed: "Deployed",
}

func (ds DeployState) String() string {
	return stateName[ds]
}
