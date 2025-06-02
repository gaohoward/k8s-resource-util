package appui

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"gaohoward.tools/k8s/resutil/pkg/common"
	"github.com/google/uuid"
)

func TestDeployOrderList(t *testing.T) {
	fakeMap, nsIds := fakeActionMap()
	result := ProcessDeployOrder(fakeMap)
	if len(result) != len(fakeMap) {
		t.Fatal("result length is nor right", "result", len(result), "map", len(fakeMap))
	}
	if len(nsIds) != 5 {
		t.Fatal("wrong number of ns ids", "len", len(nsIds))
	}

	fmt.Println("---- original map ----")
	for _, v := range fakeMap {
		fmt.Printf("value %s\n", v.Instance.GetSpecApiVer())
	}

	fmt.Println("---- After ordering ----")
	for n, r := range result {
		if act, ok := fakeMap[r]; ok {
			fmt.Printf("order %d action %s\n", n, act.Instance.GetSpecApiVer())
		}
	}

	// 6 namespces in the front
	for i := range len(nsIds) {
		if _, ok := nsIds[result[i]]; !ok {
			t.Fatal("the order is not right")
		}
	}
}

func fakeActionMap() (map[string]*common.ResourceInstanceAction, map[string]string) {
	fakeMap := make(map[string]*common.ResourceInstanceAction, 0)
	nsIds := make(map[string]string)

	for i := range 20 {
		id := uuid.NewString()
		if i%4 == 0 {
			fmt.Printf("got one at %d\n", i)
			nsIds[id] = id
			fakeMap[id] = CreateFakeAction("v1/namespaces", id)
		} else {
			fakeMap[id] = CreateFakeAction(RandomApiVer(), id)
		}
	}
	return fakeMap, nsIds
}

func CreateFakeAction(apiVer string, id string) *common.ResourceInstanceAction {
	resAction := &common.ResourceInstanceAction{
		Instance: &common.ResourceInstance{
			Id: id,
			Spec: &common.ResourceSpec{
				ApiVer: apiVer,
			},
		},
	}
	return resAction
}

func RandomApiVer() string {
	num := rand.IntN(5)

	switch num {
	case 0:
		return "v1/pods"
	case 1:
		return "apps/v1/deployments"
	case 2:
		return "v1/secrets"
	case 3:
		return "v1/configmaps"
	}
	return "apps/v1/statefulsets"
}
