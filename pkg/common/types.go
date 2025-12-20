package common

import (
	"crypto/sha256"
	"encoding/hex"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gaohoward.tools/k8s/resutil/pkg/config"
	"gaohoward.tools/k8s/resutil/pkg/graphics"
	"gaohoward.tools/k8s/resutil/pkg/resources/cached"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type ResourceAction int

const (
	Create ResourceAction = iota
	Update
	Delete
)

type BuiltinKind string

type INode interface {
	IsRoot() bool
	GetId() string
	GetParent() *Collection
	// Owner collection returns the Collection
	// That is either itself if it is a collection
	// or the direct containing collection if it
	// is a resource node (i.e. a yaml file)
	// So for a resource node it returns its
	// parent, for a collection it returns itself
	GetOwnerCollection() *Collection
	Reload(targetPath string) error
	Load(targetPath string) error
	Save(targetPath string, recursive bool) error
	FindNode(resId string) INode
	GetClickable() *widget.Clickable
	GetPath() string
	GetHolder() map[string]INode
	GetLabel() string
	GetName() string
	GetConfigContent() string
	GetResourceBag() *ResourceBag
	GetChildren() []*Collection
	GetDiscloserState() *component.DiscloserState
	GetAllResources() []string
	// remove itself from tree and delete the resource file
	Remove() error
	// all clickables must be cloned
	// the resource pointers shouldn't
	CloneForInput(newOwner *Collection, newHolder map[string]INode) INode
}

type Collection struct {
	// parentId maybe replaced with a *Collection owner
	// to be consistent with what a ResourceNode holds its owner
	parentId       string
	Id             string
	name           string
	Configuration  config.CollectionConfig
	resources      *ResourceBag
	children       []*Collection
	discloserState component.DiscloserState
	clickable      widget.Clickable
	path           string
	// needed for reload itself
	holder map[string]INode
	Dirty  bool
}

func (c *Collection) SetDirty(flag bool) {
	c.Dirty = flag
}

func (c *Collection) IsRoot() bool {
	return c.parentId == ""
}

func (c *Collection) GetDefaultNamespace() string {
	for _, entry := range c.Configuration.CollectionConfigurable.Properties {
		if entry.Name == "namespace" {
			return entry.Value
		}
	}
	//ask parent
	if parent := c.GetParent(); parent != nil {
		return parent.GetDefaultNamespace()
	}
	return config.DEFAULT_NAMESPACE
}

func (c *Collection) FindDirectResourceByName(name string) *ResourceNode {
	if resNode := c.resources.FindResource(name); resNode != nil {
		return resNode
	}
	return nil
}

func (c *Collection) CloneForInput(newOwner *Collection, newHolder map[string]INode) INode {
	clone := &Collection{
		parentId:      c.parentId,
		Id:            c.Id,
		name:          c.name,
		Configuration: c.Configuration,
		children:      make([]*Collection, 0),
		path:          c.path,
		holder:        newHolder,
	}
	clone.resources = c.resources.Clone(c, newHolder)
	for _, ch := range c.children {
		chClone := ch.CloneForInput(c, newHolder)
		if chCol, ok := chClone.(*Collection); ok {
			clone.children = append(clone.children, chCol)
		}
	}
	newHolder[clone.Id] = clone
	return clone
}

func (c *Collection) GetParent() *Collection {
	if p, ok := c.holder[c.parentId]; ok {
		if pc, ok := p.(*Collection); ok {
			return pc
		}
	}
	return nil
}

func (c *Collection) Remove() error {
	delete(c.holder, c.GetId())

	if err := c.resources.RemoveAll(); err != nil {
		return err
	}

	for _, ch := range c.children {
		if err := ch.Remove(); err != nil {
			return err
		}
	}
	if len(c.children) > 0 {
		c.children = make([]*Collection, 0)
	}

	// finally remove itself
	err := os.RemoveAll(c.path)
	if err != nil {
		return err
	}

	return nil
}

func NewCollection(name string, pid *string, id *string, config *config.CollectionConfig, path string, holder map[string]INode) *Collection {
	initId := ""
	initPid := ""
	if id == nil {
		initId = uuid.New().String()
	} else {
		initId = *id
	}
	if pid == nil {
		logger.Debug("parent id is nil, could be possible for a repository", zap.String("repo", name))
	} else {
		initPid = *pid
	}
	collection := &Collection{
		parentId: initPid,
		// note: the id may be re-set during loading
		Id:            initId,
		name:          name,
		Configuration: *config,
		children:      make([]*Collection, 0),
		path:          path,
		holder:        holder,
		Dirty:         false,
	}
	collection.Configuration.Id = initId
	collection.Configuration.Name = name
	collection.resources = NewResourceBag(collection)
	holder[initId] = collection
	return collection
}

func (c *Collection) GetLabel() string {
	return c.name
}

func (c *Collection) AddResource(res *ResourceInstance) *ResourceNode {
	if res == nil {
		return nil
	}
	return c.resources.AddInstance(res)
}

func (c *Collection) NewChild(name string, config *config.CollectionConfig) *Collection {
	childPath := filepath.Join(c.GetPath(), name)
	pid := c.GetId()
	child := NewCollection(name, &pid, nil, config, childPath, c.holder)
	c.children = append(c.children, child)
	return child
}

// this returns all instances in the collection
func (c *Collection) GetAllResourceInstances() []*ResourceInstance {
	allInsts := make([]*ResourceInstance, 0)
	directInsts := c.resources.GetAllResourceInstances()
	allInsts = append(allInsts, directInsts...)
	for _, ch := range c.children {
		subRes := ch.GetAllResourceInstances()
		allInsts = append(allInsts, subRes...)
	}
	return allInsts
}

// GetAllResources implements Node.
func (c *Collection) GetAllResources() []string {
	allRes := []string{c.Id}
	res := c.resources.GetAllResources()
	allRes = append(allRes, res...)
	for _, ch := range c.children {
		subRes := ch.GetAllResources()
		allRes = append(allRes, subRes...)
	}
	return allRes
}

func (c *Collection) GetDiscloserState() *component.DiscloserState {
	return &c.discloserState
}

func (c *Collection) GetChildren() []*Collection {
	return c.children
}

func (c *Collection) GetResourceBag() *ResourceBag {
	return c.resources
}

func (c *Collection) GetName() string {
	return c.name
}

func (c *Collection) GetPath() string {
	return c.path
}

func (c *Collection) GetClickable() *widget.Clickable {
	return &c.clickable
}

func (c *Collection) GetHolder() map[string]INode {
	return c.holder
}

func (c *Collection) GetId() string {
	return c.Id
}

// GetOwnerCollection implements Node.
// For collections its owner is itself
func (c *Collection) GetOwnerCollection() *Collection {
	return c
}

func (c *Collection) Reload(targetDir string) error {
	c.resources.Clear()
	c.children = make([]*Collection, 0)
	return c.Load(targetDir)
}

func (c *Collection) Load(targetDir string) error {

	//self
	c.holder[c.GetId()] = c

	realDir := c.GetPath()

	if targetDir != "" {
		realDir = targetDir
	}

	// The collections dir contains resource collections
	// each collection is a directory under this dir
	// The name is the dir name. In it a txt file
	// called 'desc' and a list of resource yamls
	// whose names are their resource names (without .yaml)
	// and a list of sub-collections

	entries, err := os.ReadDir(realDir)
	if err != nil {
		return err
	}

	hasConfig := false
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			if c.ignoreDir(name) {
				continue
			}
			// child collection
			child := c.NewChild(name, &config.CollectionConfig{})
			err := child.Load("")
			if err != nil {
				return err
			}
		} else {
			if name == DESC_EXT {
				data, err := os.ReadFile(filepath.Join(realDir, name))
				if err != nil {
					return err
				}

				config, err := ParseCollectionConfig(data)
				if err != nil {
					return err
				}

				hasConfig = true

				c.Configuration = *config

				realId := config.Id

				if c.Id != realId {
					delete(c.holder, c.Id)
					c.Id = realId
					c.holder[realId] = c
				}

			} else if strings.HasSuffix(name, ".yaml") {
				instName := NameFromYaml(name)
				resInstance := InstanceFromYAML(filepath.Join(realDir, name), instName)
				c.AddResource(resInstance)
			}
		}
	}
	// sort the resources
	c.GetResourceBag().Sort()
	if !hasConfig {
		c.Configuration = *CreateCollectionConfig(c.name, c.Id, c.Configuration.Description)
		realId := c.Configuration.Id
		if c.Id != realId {
			delete(c.holder, c.Id)
			c.Id = realId
			c.holder[realId] = c
		}
		c.Save("", false)
	}

	return nil
}

func (c *Collection) ignoreDir(name string) bool {
	return name == ".git"
}

// if you pass a targetDir, the collection won't
// create its dir. Instead it will save its sub
// contents (i.e. resources and sub-collection children)
// to the targetDir.
func (c *Collection) Save(targetDir string, recursive bool) error {
	//first save self
	realTarget := targetDir
	if targetDir == "" {
		realTarget = c.GetPath()
		err := os.MkdirAll(realTarget, 0755)
		if err != nil {
			return err
		}
	}

	//now create a desc file
	descPath := filepath.Join(realTarget, DESC_EXT)
	content, err := yaml.Marshal(c.Configuration)
	if err != nil {
		return err
	}
	err = os.WriteFile(descPath, content, 0644)
	if err != nil {
		return err
	}

	if !recursive {
		return nil
	}

	//then its own resources
	err = c.resources.Save(realTarget)
	if err != nil {
		return err
	}
	//then all sub resources
	for _, ch := range c.children {
		err = ch.Save("", recursive)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Collection) GetConfigContent() string {
	content, err := yaml.Marshal(&c.Configuration.CollectionConfigurable)
	if err != nil {
		return err.Error()
	}

	return string(content)
}

func (c *Collection) FindNode(resId string) INode {
	if c.resources.owner.GetId() == resId {
		return c
	}
	for _, n := range c.resources.ResourceNodes {
		if n.Instance.GetId() == resId {
			return n
		}
	}
	for _, cd := range c.children {
		n := cd.FindNode(resId)
		if n != nil {
			return n
		}
	}
	return nil
}

type CrInstance struct {
	Cr      string
	ShaHash string // used to compare
	FinalNs string // target namespace, set when deployed
}

func (c *CrInstance) Same(newCr *CrInstance) bool {
	return c.ShaHash == newCr.ShaHash
}

func NewCrInstance(cr string) *CrInstance {
	hash := sha256.Sum256([]byte(cr))
	return &CrInstance{
		Cr:      cr,
		ShaHash: hex.EncodeToString(hash[:]),
		FinalNs: "",
	}
}

type ResourceInstanceAction struct {
	Instance *ResourceInstance `yaml:"instance,omitempty"`
	//Note the action is based on the same kind of resources
	//User shouldn't change the apiVersion/Kind once resource is created
	Action    ResourceAction
	DefaultNs string
}

func (r *ResourceInstanceAction) GetDefaultNamespace() string {
	if r.DefaultNs != "" {
		return r.DefaultNs
	}
	return config.DEFAULT_NAMESPACE
}

func (r *ResourceInstanceAction) SetAction(action ResourceAction) {
	r.Action = action
}

func (r *ResourceInstanceAction) GetAction() ResourceAction {
	return r.Action
}

func (r *ResourceInstanceAction) GetName() string {
	return r.Instance.GetName()
}

type Resource interface {
	GetId() string
	SetId(Id string)
	IsSpecLoaded() bool
	GetSpecApiVer() string
	SetSpecSchema(crd string)
	SetSpecLoaded(loaded bool)
	GetCR() string
	SetCR(cr string)
	GetSpecSchema() string
	GetLabel() string
	GetName() string
	Update(node INode)
	MarkDirty(flag bool)
	IsDirty() bool
}

type ResourceNode struct {
	// Owner could be nil if not loaded from a repository
	Owner     *Collection
	Path      string
	Instance  *ResourceInstance
	clickable widget.Clickable
}

func (r *ResourceNode) IsRoot() bool {
	return false
}

func (r *ResourceNode) GetDefaultNamespace() string {
	if r.Owner != nil {
		return r.Owner.GetDefaultNamespace()
	}
	return config.DEFAULT_NAMESPACE
}

// create a standalone (no owner, path and clickable) node
func NewResourceNode(inst *ResourceInstance) *ResourceNode {
	rnode := &ResourceNode{
		Owner:    nil,
		Path:     "",
		Instance: inst,
	}
	return rnode
}

func (r *ResourceNode) CloneForInput(newOwner *Collection, newHolder map[string]INode) INode {
	return &ResourceNode{
		Owner:    newOwner,
		Path:     r.Path,
		Instance: r.Instance,
	}
}

func (r *ResourceNode) GetParent() *Collection {
	return r.Owner
}

func (r *ResourceNode) Remove() error {
	delete(r.Owner.GetHolder(), r.Instance.GetId())
	r.Owner.resources.RemoveReorder(r.Instance.GetId())
	r.Instance = nil
	return os.Remove(r.Path)
}

func (r *ResourceNode) FindNode(id string) INode {
	if r.Instance.GetId() == id {
		return r
	}
	return nil
}

// Todo: We don't want to save the FullSpec and Schema
// parts. Maybe find a yaml package that allow control
// over which fields can be ignored in Marshal/Unmarshal.
func (r *ResourceNode) Save(path string, recursive bool) error {
	realPath := r.Path
	if path != "" {
		realPath = path
	}

	data, err := yaml.Marshal(r.Instance)
	if err != nil {
		return err
	}
	return os.WriteFile(realPath, []byte(data), 0644)
}

// the load is not used for single resources
func (r *ResourceNode) Load(targetPath string) error {
	return nil
}

func (r *ResourceNode) GetResourceBag() *ResourceBag {
	return nil
}

func (r *ResourceNode) GetHolder() map[string]INode {
	return r.Owner.holder
}

func (r *ResourceNode) GetDiscloserState() *component.DiscloserState {
	return nil
}

func (r *ResourceNode) GetConfigContent() string {
	//not really used
	return r.Instance.GetLabel()
}

func (r *ResourceNode) GetClickable() *widget.Clickable {
	return &r.clickable
}

func (r *ResourceNode) GetChildren() []*Collection {
	return nil
}

func (r *ResourceNode) GetId() string {
	return r.Instance.Id
}

func (r *ResourceNode) GetOwnerCollection() *Collection {
	return r.Owner
}

func (r *ResourceNode) Reload(targetPath string) error {
	newInstance := InstanceFromYAML(r.Path, r.Instance.GetName())
	r.Instance.UpdateWith(newInstance)
	return nil
}

func (r *ResourceNode) GetPath() string {
	return r.Path
}

func (r *ResourceNode) GetName() string {
	return r.Instance.GetName()
}

// GetAllResources implements INode.
func (r *ResourceNode) GetAllResources() []string {
	return []string{r.Instance.Id}
}

func (r *ResourceNode) GetLabel() string {
	return r.Instance.GetLabel()
}

func (r *ResourceNode) GetResource() Resource {
	return r.Instance
}

type ResourceBag struct {
	owner         *Collection
	ResourceNodes []*ResourceNode
}

func (rb *ResourceBag) SetInstanceAt(pos int, res *ResourceNode) {
	*res.Instance.Order = pos
	rb.ResourceNodes[pos] = res
}

func (rb *ResourceBag) RemoveReorder(removedId string) {
	//because we use sequential int order, need to resort
	newNodes := make([]*ResourceNode, 0)
	reachRemoved := false
	for i, rn := range rb.ResourceNodes {
		if rn.Instance.GetId() != removedId {
			if reachRemoved {
				*rn.Instance.Order = i - 1
			}
			newNodes = append(newNodes, rn)
		} else {
			reachRemoved = true
		}
	}
	rb.ResourceNodes = newNodes
	rb.owner.Save("", true)
}

func (rb *ResourceBag) Sort() {
	slices.SortFunc(rb.ResourceNodes, func(a, b *ResourceNode) int {

		if *a.Instance.Order > *b.Instance.Order {
			return 1
		} else if *a.Instance.Order == *b.Instance.Order {
			return 0
		} else {
			return -1
		}
	})
}

func (rb *ResourceBag) FindResource(name string) *ResourceNode {
	for _, res := range rb.ResourceNodes {
		if res.Instance.GetName() == name {
			return res
		}
	}
	return nil
}

func (rb *ResourceBag) GetAllResourceInstances() []*ResourceInstance {
	allInsts := make([]*ResourceInstance, 0)
	for _, inst := range rb.ResourceNodes {
		allInsts = append(allInsts, inst.Instance)
	}
	return allInsts
}

func (rb *ResourceBag) Clone(newOwner *Collection, newHolder map[string]INode) *ResourceBag {
	newRb := &ResourceBag{
		owner:         newOwner,
		ResourceNodes: make([]*ResourceNode, 0),
	}
	for _, rn := range rb.ResourceNodes {
		newClone := rn.CloneForInput(newOwner, newHolder)
		if newRn, ok := newClone.(*ResourceNode); ok {
			newRb.ResourceNodes = append(newRb.ResourceNodes, newRn)
		}
	}
	return newRb
}

func (rb *ResourceBag) RemoveAll() error {
	for _, rn := range rb.ResourceNodes {
		if err := rn.Remove(); err != nil {
			return err
		}
	}
	rb.ResourceNodes = []*ResourceNode{}
	return nil
}

func NewResourceBag(owner *Collection) *ResourceBag {
	bag := ResourceBag{
		owner:         owner,
		ResourceNodes: make([]*ResourceNode, 0),
	}
	return &bag
}

func (rb *ResourceBag) GetAllResources() []string {
	res := make([]string, 0)
	for _, rn := range rb.ResourceNodes {
		res = append(res, rn.GetId())
	}
	return res
}

func (rb *ResourceBag) Clear() {
	rb.ResourceNodes = make([]*ResourceNode, 0)
}

func (rb *ResourceBag) SetOwner(o *Collection) {
	rb.owner = o
}

func (rb *ResourceBag) Save(path string) error {
	var err error = nil
	for _, rn := range rb.ResourceNodes {
		targetPath := filepath.Join(path, rn.GetName()+".yaml")
		err = rn.Save(targetPath, false)
		if err != nil {
			return err
		}
	}
	return err
}

func (r *ResourceBag) AddInstance(res *ResourceInstance) *ResourceNode {
	resNode := &ResourceNode{
		Instance: res,
		Owner:    r.owner,
		Path:     filepath.Join(r.owner.GetPath(), res.GetName()+".yaml"),
	}
	r.ResourceNodes = append(r.ResourceNodes, resNode)

	r.owner.GetHolder()[res.GetId()] = resNode
	return resNode
}

type ResourceInstance struct {
	Id       string        `yaml:"id,omitempty"`
	Spec     *ResourceSpec `yaml:"spec,omitempty"`
	Cr       string        `yaml:"cr,omitempty"`
	Order    *int          `yaml:"order,omitempty"`
	InstName string
	Label    string
	Dirty    bool
}

// IsDirty implements Resource.
func (ri *ResourceInstance) IsDirty() bool {
	return ri.Dirty
}

// MarkDirty implements Resource.
func (ri *ResourceInstance) MarkDirty(flag bool) {
	ri.Dirty = flag
}

func (ri *ResourceInstance) Clone() *ResourceInstance {
	clone := &ResourceInstance{
		Id: "",
		Spec: &ResourceSpec{
			ApiVer: ri.Spec.ApiVer,
			Schema: ri.Spec.Schema,
			Loaded: ri.Spec.Loaded,
		},
		Cr:    ri.Cr,
		Order: ri.Order,
	}
	return clone
}

func (ri *ResourceInstance) SetName(newName string) {
	ri.InstName = newName
}

// SetCR implements Resource.
func (ri *ResourceInstance) SetCR(cr string) {
	ri.Cr = cr
}

// Update implements Resource.
func (ri *ResourceInstance) Update(node INode) {

	if inst, ok := node.(*ResourceNode); ok {
		ri.UpdateWith(inst.Instance)
	} else {
		logger.Warn("The node is not a ResourceNode", zap.String("node", node.GetName()))
	}
}

func (ri *ResourceInstance) UpdateWith(newInstance *ResourceInstance) {
	if newInstance != nil {
		ri.Spec = newInstance.Spec
		ri.Cr = newInstance.Cr
		ri.Order = newInstance.Order
	}
}

func (ri *ResourceInstance) SetId(Id string) {
	ri.Id = Id
}

func (ri *ResourceInstance) GetId() string {
	return ri.Id
}

func (ri *ResourceInstance) IsSpecLoaded() bool {
	if ri.Spec.Loaded != nil {
		return *ri.Spec.Loaded
	}
	return false
}

func (ri *ResourceInstance) GetSpecApiVer() string {
	return ri.Spec.ApiVer
}

func (ri *ResourceInstance) SetSpecSchema(crd string) {
	ri.Spec.Schema = crd
}

func (ri *ResourceInstance) SetSpecLoaded(loaded bool) {
	ri.Spec.Loaded = &loaded
}

func (ri *ResourceInstance) GetCR() string {
	return ri.Cr
}

func (ri *ResourceInstance) GetSpecSchema() string {
	return ri.Spec.Schema
}

func (ri *ResourceInstance) GetLabel() string {
	return ri.Label
}

func (ri *ResourceInstance) GetName() string {
	return ri.InstName
}

type ResourceSpec struct {
	// e.g. v1/pods
	ApiVer string `yaml:"apiVer,omitempty"`
	Schema string
	Loaded *bool
}

func (rs *ResourceSpec) GetSchema() string {
	return rs.Schema
}

func InstanceFromYAML(path string, name string) *ResourceInstance {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Error reading YAML file: %v", err)
		return nil
	}

	var instance ResourceInstance
	err = yaml.Unmarshal(data, &instance)
	if err != nil {
		logger.Warn("Error unmarshalling", zap.String("file", path), zap.String("name", name), zap.Error(err))
		return nil
	}

	instance.InstName = name
	instance.Label = name

	instance.Spec.Schema = GetResSpecSchema(instance.Spec.ApiVer)

	return &instance
}

const (
	POD         BuiltinKind = "POD"
	CONFIGMAP   BuiltinKind = "CONFIGMAP"
	SECRET      BuiltinKind = "SECRET"
	SERVICE     BuiltinKind = "SERVICE"
	PV          BuiltinKind = "PV"
	PVC         BuiltinKind = "PVC"
	STATEFULSET BuiltinKind = "STATEFULSET"
	DEPLOYMENT  BuiltinKind = "DEPLOYMENT"
	INGRESS     BuiltinKind = "INGRESS"
	COLLECTION  BuiltinKind = "Collection"
)

type ApiVerGvName struct {
	ApiVer string
	Gv     string
	Name   string
}

func NewApiVerGvName(s1, s2, s3 string) *ApiVerGvName {
	return &ApiVerGvName{
		ApiVer: s1,
		Gv:     s2,
		Name:   s3,
	}
}

var ApiVersionMap = map[BuiltinKind]*ApiVerGvName{
	POD:         NewApiVerGvName("v1/pods", "v1", "pods"),
	CONFIGMAP:   NewApiVerGvName("v1/configmaps", "v1", "configmaps"),
	SECRET:      NewApiVerGvName("v1/secrets", "v1", "secrets"),
	SERVICE:     NewApiVerGvName("v1/services", "v1", "services"),
	PV:          NewApiVerGvName("v1/persistentvolumes", "v1", "persistentvolumes"),
	PVC:         NewApiVerGvName("v1/persistentvolumeclaims", "v1", "persistentvolumeclaims"),
	STATEFULSET: NewApiVerGvName("apps/v1/statefulsets", "apps/v1", "statefulsets"),
	DEPLOYMENT:  NewApiVerGvName("apps/v1/deployments", "apps/v1", "deployments"),
	INGRESS:     NewApiVerGvName("networking.k8s.io/v1/ingresses", "networking.k8s.io/v1", "ingresses"),
	COLLECTION:  NewApiVerGvName("", "", ""),
}

var PossibleUserInputMap = map[string]BuiltinKind{
	"pod":                    POD,
	"pods":                   POD,
	POD.ToApiVer():           POD,
	"configmap":              CONFIGMAP,
	"configmaps":             CONFIGMAP,
	CONFIGMAP.ToApiVer():     CONFIGMAP,
	"secret":                 SECRET,
	"secrets":                SECRET,
	SECRET.ToApiVer():        SECRET,
	"service":                SERVICE,
	"services":               SERVICE,
	SERVICE.ToApiVer():       SERVICE,
	"pv":                     PV,
	"pvs":                    PV,
	"persistentvolume":       PV,
	"persistentvolumes":      PV,
	PV.ToApiVer():            PV,
	"pvc":                    PVC,
	"pvcs":                   PVC,
	"persistentvolumeclaim":  PVC,
	"persistentvolumeclaims": PVC,
	PVC.ToApiVer():           PVC,
	"statefulset":            STATEFULSET,
	"statefulsets":           STATEFULSET,
	STATEFULSET.ToApiVer():   STATEFULSET,
	"deployment":             DEPLOYMENT,
	"deployments":            DEPLOYMENT,
	DEPLOYMENT.ToApiVer():    DEPLOYMENT,
	"ingress":                INGRESS,
	"ingresses":              INGRESS,
	INGRESS.ToApiVer():       INGRESS,
}

func (b BuiltinKind) ToApiVer() string {
	if ap, ok := ApiVersionMap[b]; ok {
		return ap.ApiVer
	}
	return ""
}

func (b BuiltinKind) ToGroupVersion() string {
	if ap, ok := ApiVersionMap[b]; ok {
		return ap.Gv
	}
	return ""
}

func (b BuiltinKind) ToApiName() string {
	if ap, ok := ApiVersionMap[b]; ok {
		return ap.Name
	}
	return ""
}

// userInput could be built-in names like pod
// or formal ones like v1/pods
func IsBuiltinTypeSupported(userInput string) (bool, string) {
	input := strings.ToLower(userInput)
	if builtinKind, ok := PossibleUserInputMap[input]; ok {
		return true, builtinKind.ToApiVer()
	}
	return false, ""
}

func NewBuiltinInstance(apiVer string, order int) *ResourceInstance {
	res := ResourceInstance{
		Spec:     GetBuiltinResSpec(apiVer),
		InstName: "",
		Cr:       SampleCrs[apiVer],
		Order:    new(int),
		Dirty:    false,
	}
	res.Order = &order
	res.SetId(apiVer)
	res.Label = string(PossibleUserInputMap[apiVer])
	return &res
}

var SampleCrs = map[string]string{
	POD.ToApiVer():         cached.PodCr,
	STATEFULSET.ToApiVer(): cached.StatefulSetCr,
	DEPLOYMENT.ToApiVer():  cached.DeploymentCr,
	SERVICE.ToApiVer():     cached.ServiceCr,
	SECRET.ToApiVer():      cached.SecretCr,
	CONFIGMAP.ToApiVer():   cached.ConfigMapCr,
	PV.ToApiVer():          cached.PvCr,
	PVC.ToApiVer():         cached.PvcCr,
	INGRESS.ToApiVer():     cached.IngressCr,
}

func GetResSpecSchema(apiVer string) string {
	spec, ok := BuiltinResSpecMap[apiVer]
	if ok {
		return spec.Schema
	}
	return ""
}

var BuiltinResSpecMap = map[string]ResourceSpec{
	POD.ToApiVer(): {
		ApiVer: POD.ToApiVer(),
		Schema: cached.PodSchema,
	},
	CONFIGMAP.ToApiVer(): {
		ApiVer: CONFIGMAP.ToApiVer(),
		Schema: cached.ConfigMapSchema,
	},
	SECRET.ToApiVer(): {
		ApiVer: SECRET.ToApiVer(),
		Schema: cached.SecretSchema,
	},
	STATEFULSET.ToApiVer(): {
		ApiVer: STATEFULSET.ToApiVer(),
		Schema: cached.StatefulSetSchema,
	},
	DEPLOYMENT.ToApiVer(): {
		ApiVer: DEPLOYMENT.ToApiVer(),
		Schema: cached.DeploymentSchema,
	},
	SERVICE.ToApiVer(): {
		ApiVer: SERVICE.ToApiVer(),
		Schema: cached.ServiceSchema,
	},
	PV.ToApiVer(): {
		ApiVer: PV.ToApiVer(),
		Schema: cached.PvSchema,
	},
	PVC.ToApiVer(): {
		ApiVer: PVC.ToApiVer(),
		Schema: cached.PvcSchema,
	},
	INGRESS.ToApiVer(): {
		ApiVer: INGRESS.ToApiVer(),
		Schema: cached.IngressSchema,
	},
}

func GetBuiltinResSpec(apiVer string) *ResourceSpec {
	spec, ok := BuiltinResSpecMap[apiVer]
	if ok {
		return &spec
	}
	return nil
}

type ResourceCollection struct {
	collection *Collection
}

// IsDirty implements Resource.
func (rcn *ResourceCollection) IsDirty() bool {
	return rcn.collection.Dirty
}

// MarkDirty implements Resource.
func (rcn *ResourceCollection) MarkDirty(flag bool) {
	rcn.collection.SetDirty(flag)
}

// SetCR implements Resource.
func (rcn *ResourceCollection) SetCR(cr string) {
	err := yaml.Unmarshal([]byte(cr), &rcn.collection.Configuration.CollectionConfigurable)
	if err != nil {
		logger.Warn("Error unmarshalling CR", zap.String("cr", cr), zap.Error(err))
		return
	}
}

// Update implements Resource.
func (rcn *ResourceCollection) Update(node INode) {
	if inst, ok := node.(*Collection); ok {
		rcn.UpdateWith(inst)
	} else {
		logger.Warn("The node is not a ResourceNode", zap.String("node", node.GetName()))
	}
}

func (rcn *ResourceCollection) UpdateWith(c *Collection) {
	rcn.collection = c
}

func (rcn *ResourceCollection) SetId(Id string) {
	rcn.collection.Id = Id
}

func (rcn *ResourceCollection) GetId() string {
	return rcn.collection.GetId()
}

func (rcn *ResourceCollection) IsSpecLoaded() bool {
	return true
}

func (rcn *ResourceCollection) GetSpecApiVer() string {
	return "No api ver for collections"
}

func (rcn *ResourceCollection) SetFullSpec(fullcr string) {
}

func (rcn *ResourceCollection) SetSpecSchema(crd string) {
	// no op
}

func (rcn *ResourceCollection) SetSpecLoaded(loaded bool) {
	// no op
}

func (rcn *ResourceCollection) GetCR() string {
	return rcn.collection.GetConfigContent()
}

func (rcn *ResourceCollection) GetConfig() *config.CollectionConfig {
	return &rcn.collection.Configuration
}

func (rcn *ResourceCollection) GetFullSpec() string {
	return "no full spec for collection"
}

func (rcn *ResourceCollection) GetSpecSchema() string {
	return "no schema for collection"
}

func (rcn *ResourceCollection) GetLabel() string {
	return rcn.collection.name
}

func (rcn *ResourceCollection) GetName() string {
	return rcn.collection.name
}

func NewResourceCollection(col *Collection) *ResourceCollection {
	rcol := ResourceCollection{
		collection: col,
	}
	return &rcol
}

type ResourceManager interface {
	GetNodeMap() map[string]INode
	SaveResource(resId string)
	SaveTemplate(current *ResourceInstance)
	IsRepo(id string) bool
}

var ItemFunc = func(gtx layout.Context, btn *widget.Clickable, text string, icon *widget.Icon) layout.Dimensions {
	th := GetTheme()
	item := component.MenuItem(th, btn, text)
	item.Icon = icon
	item.Hint = component.MenuHintText(th, "")
	return item.Layout(gtx)
}

type IResourceDetail interface {
	GetContent() layout.Widget
	GetClickable() *widget.Clickable
	GetLabel() layout.Widget
	Changed() bool
	SetSelected(state bool)
	Save(baseDir string, kind string, name string, ns string)
}

type StatusType int

func (s StatusType) String() string {
	switch s {
	case ContainerRunning:
		return "Running"
	case ContainerTerminated:
		return "Terminated"
	case ContainerTerminatedWithError:
		return "Terminated with Error"
	case ContainerError:
		return "Error"
	case ContainerUnknown:
		return "Unknown"
	case PodError:
		return "Pod Error"
	default:
		return "Unknown"
	}
}

const (
	ContainerRunning StatusType = iota
	ContainerTerminated
	ContainerTerminatedWithError
	ContainerError
	ContainerUnknown
	PodError
	PodUnknown
	PodRunning
)

type StatusIcon struct {
	Status     StatusType
	Reason     string
	icon       *widget.Icon
	color      color.NRGBA
	background color.NRGBA
	Tooltip    component.Tooltip
	TipArea    component.TipArea
	Clickable  widget.Clickable
	button     component.TipIconButtonStyle
}

func (si *StatusIcon) GetStatus() StatusType {
	return si.Status
}

func (si *StatusIcon) GetReason() string {
	return si.Reason
}

type ClusterInfo struct {
	Host string
	Id   string
}

type ResStatusInfo interface {
	SetStatus(status StatusType, reason string)
	GetStatus() StatusType
	GetReason() string
	Layout(gtx layout.Context, size unit.Dp, inset *layout.Inset) layout.Dimensions
}

type ResStatus struct {
	ResName string
	*StatusIcon
}

func (p *ResStatus) SetStatus(status StatusType, reason string) {
	p.StatusIcon = NewStatusIcon(status, reason)
}

type PodStatusInfo struct {
	*ResStatus
	ContainersInfo map[string]*PodContainerInfo
}

func (p *PodStatusInfo) SetContainerStatus(conName string, status StatusType, reason string) {
	p.ContainersInfo[conName] = &PodContainerInfo{
		Name:       conName,
		StatusIcon: NewStatusIcon(status, reason),
	}
}

func NewStatusIcon(status StatusType, reason string) *StatusIcon {
	th := GetTheme()

	var icon *widget.Icon
	var co color.NRGBA
	var bg color.NRGBA

	switch status {
	case ContainerRunning:
		icon = graphics.RunningIcon
		co = COLOR.DarkGreen
	case ContainerTerminated:
		icon = graphics.TerminatedIcon
		co = COLOR.Blue
	case ContainerTerminatedWithError:
		icon = graphics.TerminatedIcon
		co = COLOR.Red
		bg = COLOR.White
	case ContainerError:
		icon = graphics.ErrorIcon
		co = COLOR.Red
		bg = COLOR.White
	case ContainerUnknown:
		icon = graphics.UnknownIcon
		co = COLOR.Gray
	case PodError:
		icon = graphics.ErrorIcon
		co = COLOR.Red
		bg = COLOR.White
	case PodUnknown:
		icon = graphics.UnknownIcon
		co = COLOR.Gray
	case PodRunning:
		icon = graphics.RunningIcon
		co = COLOR.DarkGreen
	default:
		icon = graphics.HelpIcon
		co = COLOR.Gray
	}

	si := &StatusIcon{
		Status:     status,
		Reason:     reason,
		icon:       icon,
		color:      co,
		background: bg,
	}

	tipText := si.Reason
	if strings.TrimSpace(tipText) == "" {
		tipText = si.Status.String()
	}
	si.Tooltip = component.DesktopTooltip(th, tipText)

	si.button = component.TipIconButtonStyle{
		Tooltip:         si.Tooltip,
		IconButtonStyle: material.IconButton(th, &si.Clickable, si.icon, tipText),
		State:           &si.TipArea,
	}
	si.button.Color = si.color
	si.button.IconButtonStyle.Inset = layout.Inset{Top: 0, Bottom: 0, Left: 0, Right: 0}
	si.button.Background = COLOR.White

	return si
}

func NewPodStatusInfo(podName string) *PodStatusInfo {
	return &PodStatusInfo{
		ResStatus: &ResStatus{
			ResName:    podName,
			StatusIcon: NewStatusIcon(PodUnknown, ""),
		},
		ContainersInfo: make(map[string]*PodContainerInfo, 0),
	}
}

type PodContainerInfo struct {
	Name string
	*StatusIcon
}

func (si *StatusIcon) Layout(gtx layout.Context, size unit.Dp, inset *layout.Inset) layout.Dimensions {
	si.button.Size = size
	if inset != nil {
		return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return si.button.Layout(gtx)
		})
	}
	return layout.Inset{Top: 0, Bottom: 0, Left: 1, Right: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return si.button.Layout(gtx)
	})
}
