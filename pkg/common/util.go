package common

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"image"
	"image/color"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"gaohoward.tools/k8s/resutil/pkg/config"
	"gaohoward.tools/k8s/resutil/pkg/graphics"
	"gaohoward.tools/k8s/resutil/pkg/logs"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	k8syaml "sigs.k8s.io/yaml"
)

var logger *zap.Logger

func init() {
	logger, _ = logs.NewAppLogger("common")
}

var VerticalSplitHandler layout.Widget = func(gtx layout.Context) layout.Dimensions {
	rect := image.Rectangle{
		Max: image.Point{
			X: (gtx.Dp(unit.Dp(4))),
			Y: (gtx.Constraints.Max.Y),
		},
	}
	paint.FillShape(gtx.Ops, color.NRGBA{A: 200}, clip.Rect(rect).Op())
	return layout.Dimensions{Size: rect.Max}
}

var HorizontalSplitHandler layout.Widget = func(gtx layout.Context) layout.Dimensions {
	rect := image.Rectangle{
		Max: image.Point{
			X: (gtx.Constraints.Max.X),
			Y: (gtx.Dp(unit.Dp(4))),
		},
	}
	paint.FillShape(gtx.Ops, color.NRGBA{A: 200}, clip.Rect(rect).Op())
	return layout.Dimensions{Size: rect.Max}
}

func CreateCollectionConfig(name string, id string, desc string) *config.CollectionConfig {
	if id == "" {
		id = uuid.New().String()
	}
	cfg := &config.CollectionConfig{
		Name:       name,
		Id:         id,
		Attributes: config.CollectionAttributes{},
		CollectionConfigurable: config.CollectionConfigurable{
			Description: "This is a collection of resources",
			Properties: []config.NamedValue{
				{
					Name:  "namespace",
					Value: "default",
				},
			},
		},
	}
	if desc != "" {
		cfg.Description = desc
	}
	return cfg
}

// Parse the config file for a collection
// A desc file should have its first line to be the collection's id
// the rest of the content will be returned as description
func ParseCollectionConfig(content []byte) (*config.CollectionConfig, error) {

	config := &config.CollectionConfig{}
	err := yaml.Unmarshal(content, config)

	return config, err
}

func ExtractNameFromPath(inPath string) (string, string) {
	cleanPath := strings.TrimSpace(inPath)

	if strings.HasSuffix(cleanPath, "/") {
		return "", cleanPath
	}
	parts := strings.Split(inPath, "/")
	name := parts[len(parts)-1]
	path := strings.Join(parts[:len(parts)-1], "/")

	return name, path
}

func MapToKeysString(theMap map[string]types.NamespacedName) string {
	keys := make([]string, 0, len(theMap))
	for k := range theMap {
		keys = append(keys, k)
	}
	return strings.Join(keys, ",")
}

type ApiResourceEntry struct {
	ApiVer string
	Gv     string
	ApiRes *v1.APIResource
	Schema string
}

func (are *ApiResourceEntry) Key() string {
	return are.ApiVer + are.ApiRes.Name
}

func (are *ApiResourceEntry) GetFileName() string {
	fn := strings.ReplaceAll(are.ApiVer, "/", "_")
	return fn + ".schema"
}

func (are *ApiResourceEntry) GetApiPath() string {
	if are.Gv == "v1" {
		return "api/v1"
	}
	return "apis/" + are.Gv
}

// file name is like apps_v1_statefulset.schema
func SaveSchema(entry *ApiResourceEntry, groupDir string) error {
	file := filepath.Join(groupDir, entry.GetFileName())

	os.WriteFile(file, []byte(entry.Schema), 0644)
	return nil
}

func LoadSchema(groupDir string, entry *ApiResourceEntry) error {
	file := filepath.Join(groupDir, entry.GetFileName())

	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	entry.Schema = string(data)

	return nil
}

type ApiResourceInfo struct {
	Cached  bool
	ResList []*v1.APIResourceList
	// key: v1/pods, apps/v1/statefulsets...
	ResMap map[string]*ApiResourceEntry
}

func (a *ApiResourceInfo) FindApiResource(apiVer string) *ApiResourceEntry {
	if r, ok := a.ResMap[apiVer]; ok {
		return r
	}
	return nil
}

// rt could be like v1/pods or apps/v1/statefulsets
func (a *ApiResourceInfo) HasResourceType(rt string) bool {
	for _, arl := range a.ResList {
		gv := arl.GroupVersion
		for _, res := range arl.APIResources {
			gvn := gv + "/" + res.Name
			if gvn == rt {
				return true
			}
		}
	}
	return false
}

type ApiResourcePersister interface {
	Load() (*ApiResourceInfo, error)
	Save(*ApiResourceInfo) error
}

type FileApiResourcePersister struct {
	filePath  string
	schemaDir string
	cache     []*v1.APIResourceList
}

// Save implements ApiResourcePersister.
func (f *FileApiResourcePersister) Save(apiInfo *ApiResourceInfo) error {
	if apiInfo == nil {
		return nil
	}
	data, err := yaml.Marshal(apiInfo.ResList)
	if err != nil {
		return fmt.Errorf("failed to marshal api resources: %w", err)
	}

	err = os.WriteFile(f.filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write api resources to file: %w", err)
	}

	//persist schema
	// Save the API resources to the config directory
	for _, resList := range apiInfo.ResList {
		groupDir := filepath.Join(f.schemaDir, resList.GroupVersion)
		if err := os.MkdirAll(groupDir, 0755); err == nil {
			for key, value := range apiInfo.ResMap {
				if err := SaveSchema(value, groupDir); err != nil {
					logger.Error("Failed to save schema for resource", zap.String("key", key), zap.Error(err))
				}
			}
		} else {
			logger.Warn("Cannot mkdir for group", zap.String("dir", groupDir))
		}
	}

	return nil
}

func GetCachedApiResourceList() (*ApiResourceInfo, error) {
	persister := GetApiResourcePersister()
	return persister.Load()
}

func (f *FileApiResourcePersister) Load() (*ApiResourceInfo, error) {
	result := &ApiResourceInfo{
		ResList: make([]*v1.APIResourceList, 0),
		ResMap:  make(map[string]*ApiResourceEntry),
	}
	if f.cache == nil {
		f.cache = make([]*v1.APIResourceList, 0)
		data, err := os.ReadFile(f.filePath)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, &f.cache); err != nil {
			logger.Info("failed to unmarshal api-resource data", zap.String("data", string(data)))
			return nil, err
		}
	}

	result.ResList = f.cache

	// now loading schema and populate the map
	for _, res := range result.ResList {
		for _, apiRes := range res.APIResources {
			key := res.GroupVersion + "/" + apiRes.Name
			entry := &ApiResourceEntry{
				ApiVer: key,
				Gv:     res.GroupVersion,
				ApiRes: &apiRes,
			}

			groupDir := filepath.Join(f.schemaDir, entry.Gv)

			LoadSchema(groupDir, entry)

			result.ResMap[key] = entry
		}
	}

	return result, nil
}

func GetApiResourcePersister() ApiResourcePersister {
	cfgDir, err := config.GetConfigDir()
	if err != nil {
		logger.Warn("Cannot get config dir", zap.Error(err))
		return &DummyApiResourcePersister{}
	}
	path := filepath.Join(cfgDir, "api-resources")
	if err := os.MkdirAll(path, 0755); err != nil {
		logger.Warn("Cannot make api-resources dir", zap.Error(err))
		return &DummyApiResourcePersister{}
	}
	fpath := filepath.Join(path, "apis.yaml")

	schemaPath := filepath.Join(cfgDir, "schemas")
	if err := os.MkdirAll(schemaPath, 0755); err != nil {
		logger.Warn("Cannot make schema dir", zap.Error(err))
		return &DummyApiResourcePersister{}
	}

	persister := &FileApiResourcePersister{
		filePath:  fpath,
		schemaDir: schemaPath,
	}
	return persister
}

type DummyApiResourcePersister struct {
}

// Save implements ApiResourcePersister.
func (d *DummyApiResourcePersister) Save(*ApiResourceInfo) error {
	return nil
}

// Load implements ApiResourcePersister.
func (d *DummyApiResourcePersister) Load() (*ApiResourceInfo, error) {
	return nil, nil
}

func GetAllUnstructuredItems(data []*unstructured.UnstructuredList) []*unstructured.Unstructured {
	items := make([]*unstructured.Unstructured, 0)
	for _, l := range data {
		for _, i := range l.Items {
			items = append(items, &i)
		}
	}
	return items
}

// this func move an element of a clice at [fromIndex] to [toIndex]
// while keep the order of all the rest
// for example a slice {0, 1, 2, 3, 4} if we want to move 3 to 0
// the slice would be {3, 0, 1, 2, 4}, and if we ant to move 0 to 3
// the slice would be {1, 2, 3, 0, 4}
func ReorderSlice[E any](targetSlice []E, fromIndex int, toIndex int) {
	fromItem := targetSlice[fromIndex]
	toItem := targetSlice[toIndex]
	//first move the item to its target position
	targetSlice[toIndex] = fromItem
	if fromIndex > toIndex {
		for i := fromIndex; i > toIndex; i-- {
			if i == toIndex+1 {
				targetSlice[i] = toItem
			} else {
				targetSlice[i] = targetSlice[i-1]
			}
		}
	} else if fromIndex < toIndex {
		for i := fromIndex; i < toIndex; i++ {
			if i == toIndex-1 {
				targetSlice[i] = toItem
			} else {
				targetSlice[i] = targetSlice[i+1]
			}
		}
	}
}

// Save should create a unique name like pod_pod1_default_detailtype_timestamp.ext
func CreateFilePathForK8sObject(baseDir string, kind, name, ns, category, ext string) string {
	timestamp := time.Now().Format("20060102150405")
	fileName := kind + "_" + name + "_" + ns + "_" + category + "_" + timestamp + "." + ext
	return path.Join(baseDir, fileName)
}

func SaveFile(filePath string, content *string) error {
	if content == nil {
		return fmt.Errorf("content is nil")
	}
	return os.WriteFile(filePath, []byte(*content), 0644)
}

func MarshalYaml(item *unstructured.Unstructured) (string, error) {

	bytes, err := k8syaml.Marshal(item)

	if err != nil {
		return "", err
	}

	return string(bytes), err
}

func GetAboutWidth(gtx layout.Context, th *material.Theme, headline string) layout.Dimensions {

	macro := op.Record(gtx.Ops)
	label := material.Body1(th, headline)
	label.TextSize = unit.Sp(16)
	label.Font.Weight = font.Bold
	size := label.Layout(gtx)
	macro.Stop()

	return size

}

type SearchBar struct {
	searchArea    widget.Editor
	caseSensitive bool
	ownerId       string
}

func (sb *SearchBar) IsCaseSensitive() bool {
	return sb.caseSensitive
}

func NewSearchBar(ownerId string) *SearchBar {
	return &SearchBar{
		ownerId: ownerId,
	}
}

func (sb *SearchBar) Changed(gtx layout.Context) bool {
	changed := false
	for {
		evt, ok := sb.searchArea.Update(gtx)
		if !ok {
			break
		}
		if _, isChange := evt.(widget.ChangeEvent); isChange {
			changed = true
		}
	}
	if !changed {
		if caseFlagChanged, _ := FlipContextBool(sb.ownerId); caseFlagChanged {
			changed = true
		}
	}
	return changed
}

func (sb *SearchBar) GetText() string {
	return sb.searchArea.Text()
}

func (sb *SearchBar) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	editor := material.Editor(th, &sb.searchArea, "search text")
	editor.Font.Weight = font.Bold
	editor.Color = COLOR.Blue

	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return graphics.SearchIcon.Layout(gtx, COLOR.Blue)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {

			iconColor := COLOR.LightGray

			macro := op.Record(gtx.Ops)
			size := graphics.CaseSensitiveIcon.Layout(gtx, iconColor)
			macro.Stop()

			r := image.Rectangle{Max: image.Point{X: size.Size.X, Y: size.Size.Y}}

			area := clip.Rect(r).Push(gtx.Ops)

			for {
				_, ok := gtx.Event(pointer.Filter{
					Target: sb,
					Kinds:  pointer.Press,
				})

				if !ok {
					break
				}
				sb.caseSensitive = !sb.caseSensitive
				SetContextBool(sb.ownerId, true, nil)
			}
			event.Op(gtx.Ops, sb)
			defer area.Pop()

			if sb.caseSensitive {
				iconColor = COLOR.Black
				editor.Hint = "search text (case sensitive)"
			}

			return graphics.CaseSensitiveIcon.Layout(gtx, iconColor)
		}),
		layout.Flexed(1.0, editor.Layout),
	)
}

func ParseCerts(certData []byte) ([]*x509.Certificate, error) {
	var certList = make([]*x509.Certificate, 0)
	certBlock, rest := pem.Decode(certData)
	for certBlock != nil {
		cert, err := x509.ParseCertificate(certBlock.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse certificate: %v", err)
		}
		certList = append(certList, cert)
		certBlock, rest = pem.Decode(rest)
	}
	return certList, nil
}
