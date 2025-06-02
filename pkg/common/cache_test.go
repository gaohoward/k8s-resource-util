package common

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFileDeploymentPersister(t *testing.T) {
	tempFile := "test_deployments.yaml"
	defer os.Remove(tempFile)

	t.Run("Add Deployment", func(t *testing.T) {
		persister := &FileDeploymentPersister{filePath: tempFile}
		deployment := &DeployDetail{
			Id:   "test-id",
			Name: "test-deployment",
		}

		err := persister.Add(deployment)
		if err != nil {
			t.Fatalf("failed to add deployment: %v", err)
		}

		data, err := os.ReadFile(tempFile)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}

		var loadedDeployments []*DeployDetail
		err = yaml.Unmarshal(data, &loadedDeployments)
		if err != nil {
			t.Fatalf("failed to unmarshal YAML: %v", err)
		}

		if len(loadedDeployments) != 1 || loadedDeployments[0].Id != "test-id" {
			t.Errorf("unexpected deployments in file: %v", loadedDeployments)
		}
	})

	t.Run("Update Deployment", func(t *testing.T) {
		persister := &FileDeploymentPersister{filePath: tempFile}
		deployment := &DeployDetail{
			Id:   "test-id",
			Name: "updated-deployment",
		}
		persister.cache = []*DeployDetail{deployment}

		err := persister.Update()
		if err != nil {
			t.Fatalf("failed to update deployment: %v", err)
		}

		data, err := os.ReadFile(tempFile)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}

		var loadedDeployments []*DeployDetail
		err = yaml.Unmarshal(data, &loadedDeployments)
		if err != nil {
			t.Fatalf("failed to unmarshal YAML: %v", err)
		}

		if len(loadedDeployments) != 1 || loadedDeployments[0].Name != "updated-deployment" {
			t.Errorf("unexpected deployments in file: %v", loadedDeployments)
		}
	})

	t.Run("Remove Deployment", func(t *testing.T) {
		persister := &FileDeploymentPersister{filePath: tempFile}
		deployment := &DeployDetail{
			Id:   "test-id",
			Name: "test-deployment",
		}
		persister.cache = []*DeployDetail{deployment}

		err := persister.Remove(deployment)
		if err != nil {
			t.Fatalf("failed to remove deployment: %v", err)
		}

		data, err := os.ReadFile(tempFile)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}

		var loadedDeployments []*DeployDetail
		err = yaml.Unmarshal(data, &loadedDeployments)
		if err != nil {
			t.Fatalf("failed to unmarshal YAML: %v", err)
		}

		if len(loadedDeployments) != 0 {
			t.Errorf("expected no deployments in file, got: %v", loadedDeployments)
		}
	})

	t.Run("Load Deployments", func(t *testing.T) {
		persister := &FileDeploymentPersister{filePath: tempFile}
		deployment := &DeployDetail{
			Id:   "test-id",
			Name: "test-deployment",
		}
		persister.cache = []*DeployDetail{deployment}
		err := persister.persist()
		if err != nil {
			t.Fatalf("failed to persist deployments: %v", err)
		}

		_, err = persister.Load()
		if err != nil {
			t.Fatalf("failed to load deployments: %v", err)
		}

		if len(persister.cache) != 1 || persister.cache[0].Id != "test-id" {
			t.Errorf("unexpected deployments loaded: %v", persister.cache)
		}
	})
}
