package appui

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"gaohoward.tools/k8s/resutil/pkg/common"
	"gaohoward.tools/k8s/resutil/pkg/config"
	"go.uber.org/zap"
)

// Original auther: Copilot :)
func TestCollectionSaveAndLoad(t *testing.T) {
	// Setup temporary directory for testing
	tempDir := t.TempDir()

	repoDir := filepath.Join(tempDir, "Internal")
	os.MkdirAll(repoDir, 0755)

	//config-dir
	configDir := filepath.Join(tempDir, ".k8sconfig")
	os.MkdirAll(configDir, 0755)
	os.Setenv("K8SUTIL_CONFIG_HOME", configDir)

	apiCacheDir := filepath.Join(configDir, "api-resources")
	os.MkdirAll(apiCacheDir, 0755)
	apiCacheFile := filepath.Join(apiCacheDir, "apis.yaml")
	copyFile("../testdata/api-resources/apis.yaml", apiCacheFile)

	if _, err := os.Stat(apiCacheFile); os.IsNotExist(err) {
		t.Fatalf("Resource file for apis.yaml was not created: %v", err)
	}

	logger.Info("created api file", zap.String("file", apiCacheFile))

	common.InitK8sClient(nil)

	holder := make(map[string]common.INode)
	// Create a root collection
	repo := NewCollectionRepo("local", nil, nil, &config.CollectionConfig{}, repoDir, holder)

	// Add resources to the root collection
	res1, err := common.NewInstance(common.POD.ToApiVer(), "pod1", 0)
	if err != nil {
		t.Fatalf("Failed to create resource pod1: %v", err)
	}
	res2, err := common.NewInstance(common.SECRET.ToApiVer(), "secret1", 1)
	if err != nil {
		t.Fatalf("Failed to create resource secret1: %v", err)
	}
	repo.AddResource(res1)
	repo.AddResource(res2)

	// Add a child collection
	child1 := repo.NewChild("child1", &config.CollectionConfig{
		CollectionConfigurable: config.CollectionConfigurable{
			Description: "Child Collection1",
		},
	})

	res3, err := common.NewInstance(common.STATEFULSET.ToApiVer(), "statefulset1", 0)
	if err != nil {
		t.Fatalf("Failed to create resource statefulset1: %v", err)
	}
	res31, err := common.NewInstance(common.DEPLOYMENT.ToApiVer(), "pod1", 1)
	if err != nil {
		t.Fatalf("Failed to create resource pod1: %v", err)
	}
	child1.AddResource(res3)
	child1.AddResource(res31)

	// And another child collection
	child2 := repo.NewChild("child2", &config.CollectionConfig{
		CollectionConfigurable: config.CollectionConfigurable{
			Description: "Child Collection2",
		},
	})
	res4, err := common.NewInstance(common.CONFIGMAP.ToApiVer(), "configmap1", 0)
	if err != nil {
		t.Fatalf("Failed to create resource configmap1: %v", err)
	}
	child2.AddResource(res4)
	// and a child inside child2
	grandChild1 := child2.NewChild("grandchild1", &config.CollectionConfig{
		CollectionConfigurable: config.CollectionConfigurable{
			Description: "Grand Child Collection",
		},
	})
	res5, err := common.NewInstance(common.INGRESS.ToApiVer(), "ingress1", 1)
	if err != nil {
		t.Fatalf("Failed to create resource ingress1: %v", err)
	}
	grandChild1.AddResource(res5)
	grandChild1.AddResource(res5)

	if len(holder) != 10 {
		for key, val := range holder {
			t.Logf("key: %s, val: %v", key, val.GetLabel())
		}
		t.Fatalf("Holder map is not right: %v", len(holder))
	}

	// Save the collection
	err = repo.Save(repoDir, true)
	if err != nil {
		logger.Error("failed to save collection", zap.Error(err))
		t.Fatalf("Failed to save collection: %v", err)
	}

	rootDir := filepath.Join(tempDir, "root")
	if _, err := os.Stat(rootDir); os.IsNotExist(err) {
		//pass!
	}

	descFile := filepath.Join(repoDir, common.DESC_EXT)
	if _, err := os.Stat(descFile); os.IsNotExist(err) {
		t.Fatalf("Description file was not created: %v", err)
	}

	// Verify that the child directory was created
	childDir1 := filepath.Join(repoDir, "child1")
	if _, err := os.Stat(childDir1); os.IsNotExist(err) {
		t.Fatalf("Child directory was not created: %v", err)
	}
	descFileChild1 := filepath.Join(childDir1, common.DESC_EXT)
	if _, err := os.Stat(descFileChild1); os.IsNotExist(err) {
		t.Fatalf("desc file for child1 was not created: %v", err)
	}

	// Verify that the resource files were created
	res1File := filepath.Join(repoDir, "pod1.yaml")
	if _, err := os.Stat(res1File); os.IsNotExist(err) {
		t.Fatalf("Resource file for pod1 was not created: %v", err)
	}
	res2File := filepath.Join(repoDir, "secret1.yaml")
	if _, err := os.Stat(res2File); os.IsNotExist(err) {
		t.Fatalf("Resource file for secret1 was not created: %v", err)
	}
	res3File := filepath.Join(childDir1, "statefulset1.yaml")
	if _, err := os.Stat(res3File); os.IsNotExist(err) {
		t.Fatalf("Resource file for statefulset1 was not created: %v", err)
	}

	childDir2 := filepath.Join(repoDir, "child2")
	grandChild1Dir := filepath.Join(childDir2, "grandchild1")

	res4File := filepath.Join(childDir2, "configmap1.yaml")
	if _, err := os.Stat(res4File); os.IsNotExist(err) {
		t.Fatalf("Resource file for configmap1 was not created: %v", err)
	}
	descFileChild2 := filepath.Join(childDir2, common.DESC_EXT)
	if _, err := os.Stat(descFileChild2); os.IsNotExist(err) {
		t.Fatalf("Description file for child2 was not created: %v", err)
	}

	res5File := filepath.Join(grandChild1Dir, "ingress1.yaml")
	if _, err := os.Stat(res5File); os.IsNotExist(err) {
		t.Fatalf("Resource file for ingress1 was not created: %v", err)
	}
	descGrandChild1 := filepath.Join(grandChild1Dir, common.DESC_EXT)
	if _, err := os.Stat(descGrandChild1); os.IsNotExist(err) {
		t.Fatalf("Description file for grandChild1 was not created: %v", err)
	}

	//Now load
	holder = make(map[string]common.INode)
	repo = NewCollectionRepo("local", nil, nil, &config.CollectionConfig{}, repoDir, holder)
	repo.Load(repoDir)
	if len(holder) != 10 {
		for key, val := range holder {
			t.Logf("key: %s, val: %v", key, val.GetLabel())
		}
		t.Fatalf("Holder map should contain 10 items, got %d", len(holder))
	}

	old, err := os.ReadFile(descFileChild1)
	if err != nil {
		t.Fatalf("Failed to read to file: %v", err)
	}

	t.Logf("old %v\n", string(old))

	// Write a string to a file, this will break the yaml loading
	invalidTestContent := string(old) + "\n This is a test string."

	err = os.WriteFile(descFileChild1, []byte(invalidTestContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write to file: %v", err)
	}

	// Verify the file was written correctly
	content, err := os.ReadFile(descFileChild1)
	if err != nil {
		t.Fatalf("Failed to read the file: %v", err)
	}
	if string(content) != invalidTestContent {
		t.Fatalf("File content mismatch. Expected: %s, Got: %s", invalidTestContent, string(content))
	}

	err = repo.Reload(repoDir)
	if err == nil {
		t.Fatalf("didn't catch the yaml loading error")
	}

}

// copyDir copies a directory and its contents from src to dst.
func CopyDir(src string, dst string) error {
	// Walk through the source directory
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Construct the destination path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		// If it's a directory, create it in the destination
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// If it's a file, copy it
		return copyFile(path, dstPath)
	})
}

// copyFile copies a single file from src to dst.
func copyFile(src string, dst string) error {
	// Open the source file
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create the destination file
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Copy the file contents
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	// Copy the file permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
}
