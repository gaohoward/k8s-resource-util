package appdrawer

import (
	"gaohoward.tools/k8s/resutil/pkg/common"
	"gaohoward.tools/k8s/resutil/pkg/graphics"
	"gioui.org/x/component"
)

type AppNaviItem interface {
	GetNaviItem() component.NavItem
	GetKey() any
}

type BaseItem struct {
	navItem component.NavItem
	key     common.BuiltinKind
}

func (item *BaseItem) GetNaviItem() component.NavItem {
	return item.navItem
}

func (item *BaseItem) GetKey() any {
	return item.key
}

var (
	itemStatefulSet = BaseItem{
		key: common.STATEFULSET,
		navItem: component.NavItem{
			Name: "StatefulSet",
			Icon: graphics.ResIcon,
			Tag:  common.STATEFULSET,
		},
	}
	itemDeployment = BaseItem{
		key: common.DEPLOYMENT,
		navItem: component.NavItem{
			Name: "Deployment",
			Icon: graphics.ResIcon,
			Tag:  common.DEPLOYMENT,
		},
	}
	itemService = BaseItem{
		key: common.SERVICE,
		navItem: component.NavItem{
			Name: "Service",
			Icon: graphics.ResIcon,
			Tag:  common.SERVICE,
		},
	}
	itemIngress = BaseItem{
		key: common.INGRESS,
		navItem: component.NavItem{
			Name: "Ingress",
			Icon: graphics.ResIcon,
			Tag:  common.INGRESS,
		},
	}
	itemPv = BaseItem{
		key: common.PV,
		navItem: component.NavItem{
			Name: "Persistent Volume",
			Icon: graphics.ResIcon,
			Tag:  common.PV,
		},
	}
	itemPvc = BaseItem{
		key: common.PVC,
		navItem: component.NavItem{
			Name: "Persistent Volume Claim",
			Icon: graphics.ResIcon,
			Tag:  common.PVC,
		},
	}
	itemSecret = BaseItem{
		key: common.CONFIGMAP,
		navItem: component.NavItem{
			Name: "ConfigMap",
			Icon: graphics.ResIcon,
			Tag:  common.CONFIGMAP,
		},
	}
	itemConfigMap = BaseItem{
		key: common.SECRET,
		navItem: component.NavItem{
			Name: "Secret",
			Icon: graphics.ResIcon,
			Tag:  common.SECRET,
		},
	}
	itemPod = BaseItem{
		key: common.POD,
		navItem: component.NavItem{
			Name: "Pod",
			Icon: graphics.ResIcon,
			Tag:  common.POD,
		},
	}
)

var Items = []AppNaviItem{
	&itemPod,
	&itemStatefulSet,
	&itemDeployment,
	&itemSecret,
	&itemConfigMap,
	&itemService,
	&itemPv,
	&itemPvc,
	&itemIngress,
}
