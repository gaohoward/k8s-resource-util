package common

import "strings"

type ActionID int

const (
	DESC_EXT = ".desc"
)

type ActionHandler interface {
	Handle(id ActionID, action any)
}

// a.yaml -> a
func NameFromYaml(yamlFileName string) string {
	if strings.HasSuffix(yamlFileName, ".yaml") {
		return yamlFileName[:strings.LastIndex(yamlFileName, ".yaml")]
	}
	return yamlFileName
}

type RepoListener interface {
	PathSeleted(collectionId string, path string)
	ResourceSelected(resourceId string, resPath string)
}
