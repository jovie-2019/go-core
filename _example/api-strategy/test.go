package api_strategy

import (
	"github.com/pefish/go-core/api-session"
	"github.com/pefish/go-error"
)

type TestStrategyClass struct {

}

var TestApiStrategy = TestStrategyClass{}

func (this *TestStrategyClass) GetName() string {
	return `test`
}

func (this *TestStrategyClass) GetDescription() string {
	return `test`
}

func (this *TestStrategyClass) GetErrorCode() uint64 {
	return 1000
}

func (this *TestStrategyClass) Execute(out *api_session.ApiSessionClass, param interface{}) {

	go_error.ThrowInternal(`12345test`)
}
