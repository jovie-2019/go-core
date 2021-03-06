package global_api_strategy

import (
	api_strategy "github.com/pefish/go-core/api-strategy"
)

type InterfaceGlobalStrategy interface {
	Init(param interface{})  // 同步的初始化函数
	api_strategy.InterfaceStrategy
}
