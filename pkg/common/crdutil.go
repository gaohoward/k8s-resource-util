package common

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	discovery "k8s.io/client-go/discovery"
	diskcached "k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/openapi/cached"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/util/homedir"
	"k8s.io/kubectl/pkg/explain"
	ktlexplain "k8s.io/kubectl/pkg/explain/v2"
)

/*
Some code are borrowed from kubectl:
https://github.com/kubernetes/kubectl.git
*/

// limit the text size. fyne text widgets
// can't handle large size of text.
// we need to use some other GUI framework
// and make this util as a service (e.g. http server)
// until fyne or other go GUI framework matures
// you can give a large number when using
// other framework like gio which seems doesn't
// suffer the same issue
const MAX_TEXT_SIZE = 80000

var resUtil *ResUtil

func GetResUtil() *ResUtil {
	return resUtil
}

// config could be nil when not connected
func CreateResUtil(config *rest.Config) *ResUtil {
	if resUtil == nil {
		generator := ktlexplain.NewGenerator()
		if err := registerBuiltinTemplates(generator); err != nil {
			logger.Warn("Error registing template", zap.Error(err))
		}
		resUtil = &ResUtil{
			generator: generator,
			client:    config,
		}
	}
	return resUtil
}

type ResUtil struct {
	generator ktlexplain.Generator
	client    *rest.Config
}

func (util *ResUtil) GetCRDFor(resEntry *ApiResourceEntry) (string, error) {
	crd, err := GetCRDFor(resEntry, util.client, util.generator)
	if err != nil {
		return "", err
	}
	return crd, nil
}

func HasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func CutSuffix(s, suffix string) (before string, found bool) {
	if !HasSuffix(s, suffix) {
		return s, false
	}
	return s[:len(s)-len(suffix)], true
}

func LastIndexByteString(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// special comment, don't remove it
//
//go:embed templates/*.tmpl
var rawBuiltinTemplates embed.FS

func registerBuiltinTemplates(gen ktlexplain.Generator) error {

	files, err := rawBuiltinTemplates.ReadDir("templates")
	if err != nil {
		logger.Error("Failed to read files in templates", zap.Error(err))
		return err
	}

	for _, entry := range files {
		contents, err := rawBuiltinTemplates.ReadFile("templates/" + entry.Name())
		if err != nil {
			return err
		}

		err = gen.AddTemplate(
			strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())),
			string(contents))

		if err != nil {
			return err
		}
	}

	return nil
}

func toRESTMapper(discoveryClient discovery.CachedDiscoveryInterface) (meta.RESTMapper, error) {
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient, func(a string) {
		fmt.Println(a)
	})
	return expander, nil
}

func GetGVR(client discovery.CachedDiscoveryInterface, resEntry *ApiResourceEntry) (*schema.GroupVersionResource, []string, error) {
	var fullySpecifiedGVR schema.GroupVersionResource
	var fieldsPath []string
	var err error
	resName := resEntry.ApiRes.Name
	gv := resEntry.Gv //if empty it will be guessed
	mapper, err := toRESTMapper(client)
	if err != nil {
		logger.Error("Failed to get rest mapper", zap.Error(err))
		return nil, nil, err
	}
	if len(gv) == 0 {
		fullySpecifiedGVR, fieldsPath, err = explain.SplitAndParseResourceRequestWithMatchingPrefix(resName, mapper)
		if err != nil {
			return nil, nil, err
		}
	} else {
		fullySpecifiedGVR, fieldsPath, err = explain.SplitAndParseResourceRequest(resName, mapper)
		if err != nil {
			return nil, nil, err
		}
	}

	//outputFormat plaintext
	// Check whether the server reponds to OpenAPIV3.
	if len(gv) > 0 {
		apiVersion, err := schema.ParseGroupVersion(gv)
		if err != nil {
			return nil, nil, err
		}
		fullySpecifiedGVR.Group = apiVersion.Group
		fullySpecifiedGVR.Version = apiVersion.Version
	}
	return &fullySpecifiedGVR, fieldsPath, nil
}

func getDefaultCacheDir() string {
	if kcd := os.Getenv("KUBECACHEDIR"); kcd != "" {
		return kcd
	}

	return filepath.Join(homedir.HomeDir(), ".kube", "cache")
}

var overlyCautiousIllegalFileCharacters = regexp.MustCompile(`[^(\w/.)]`)

func computeDiscoverCacheDir(parentDir, host string) string {
	// strip the optional scheme from host if its there:
	schemelessHost := strings.Replace(strings.Replace(host, "https://", "", 1), "http://", "", 1)
	// now do a simple collapse of non-AZ09 characters.  Collisions are possible but unlikely.  Even if we do collide the problem is short lived
	safeHost := overlyCautiousIllegalFileCharacters.ReplaceAllString(schemelessHost, "_")
	return filepath.Join(parentDir, safeHost)
}

func GetCachedDiscoveryClient(config *rest.Config) (discovery.CachedDiscoveryInterface, error) {

	// config.Burst = f.discoveryBurst
	// config.QPS = f.discoveryQPS

	cacheDir := getDefaultCacheDir()

	httpCacheDir := filepath.Join(cacheDir, "http")
	discoveryCacheDir := computeDiscoverCacheDir(filepath.Join(cacheDir, "discovery"), config.Host)

	return diskcached.NewCachedDiscoveryClientForConfig(config, discoveryCacheDir, httpCacheDir, time.Duration(6*time.Hour))
}

func GetCRDFor(resEntry *ApiResourceEntry, k8sConfig *rest.Config, generator ktlexplain.Generator) (string, error) {
	if k8sConfig == nil {
		return "", fmt.Errorf("no rest client configured. Did you start the cluster?")
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(k8sConfig)
	if err != nil {
		logger.Error("error creating k8s client", zap.Error(err))
		return "", err
	}

	cachedClient, err := GetCachedDiscoveryClient(k8sConfig)
	if err != nil {
		logger.Error("error creating cached client", zap.Error(err))
		return "", err
	}

	v3client := cached.NewClient(discoveryClient.OpenAPIV3())
	// v2client := cached.NewClient(discoveryClient.WithLegacy().OpenAPIV3())

	v3paths, err := v3client.Paths()
	if err != nil {
		return "", err
	}

	var resourcePath string = resEntry.GetApiPath()

	gv, exists := v3paths[resourcePath]
	if !exists {
		return "", fmt.Errorf("couldn't found path for %s\n", resourcePath)
	}

	openAPISchemaBytes, err := gv.Schema(runtime.ContentTypeJSON)
	if err != nil {
		logger.Error("error getting schema", zap.Error(err))
		return "", nil
	}

	var parsedV3Schema map[string]any
	if err := json.Unmarshal(openAPISchemaBytes, &parsedV3Schema); err != nil {
		return "", fmt.Errorf("Error unmarshaling schema")
	}

	gvr, fieldsPath, err := GetGVR(cachedClient, resEntry)

	buf := new(bytes.Buffer)

	err = generator.Render("plaintext", parsedV3Schema, *gvr, fieldsPath, true, buf)

	if err != nil {
		return "", fmt.Errorf("error render %v\n", err)
	}
	fullSpec := buf.String()

	if len(fullSpec) < MAX_TEXT_SIZE {
		return fullSpec, nil
	}
	return fullSpec[:MAX_TEXT_SIZE] + "\n...(truncated)", nil
}
