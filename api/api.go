package api

import (
	"fmt"
	global_api_strategy "github.com/pefish/go-core/driver/global-api-strategy"
	"net/http"

	"github.com/pefish/go-application"
	api_session "github.com/pefish/go-core/api-session"
	api_strategy2 "github.com/pefish/go-core/api-strategy"
	"github.com/pefish/go-core/driver/logger"
	"github.com/pefish/go-error"
	"github.com/pefish/go-stack"
)

type Api struct {
	Description            string                       // api描述
	Path                   string                       // api路径
	IgnoreRootPath         bool                         // api路径是否忽略根路径
	IgnoreGlobalStrategies bool                         // 是否跳过全局策略
	Method                 api_session.ApiMethod        // api方法
	Strategies             []api_strategy2.StrategyData // api前置处理策略,不包含全局策略
	Params                 interface{}                  // api参数
	Return                 interface{}                  // api返回值
	Controller             ApiHandlerType               // api业务处理器
	ParamType              string                       // 参数类型。默认 application/json，可选 multipart/form-data，空表示都支持
	ReturnHookFunc         ReturnHookFuncType           // 返回前的处理函数
}

func (this *Api) GetDescription() string {
	return this.Description
}

func (this *Api) GetParamType() string {
	return this.ParamType
}

func (this *Api) GetParams() interface{} {
	return this.Params
}

type ReturnHookFuncType func(apiContext *api_session.ApiSessionClass, apiResult *ApiResult) (interface{}, *go_error.ErrorInfo)

type ApiResult struct {
	Msg         string      `json:"msg"`
	InternalMsg string      `json:"internal_msg"`
	Code        uint64      `json:"code"`
	Data        interface{} `json:"data"`
}

type ApiHandlerType func(apiSession *api_session.ApiSessionClass) interface{}

func DefaultReturnDataFunc(msg string, internalMsg string, code uint64, data interface{}) *ApiResult {
	if go_application.Application.Debug {
		return &ApiResult{
			Msg:         msg,
			InternalMsg: internalMsg,
			Code:        code,
			Data:        data,
		}

	} else {
		return &ApiResult{
			Msg:         msg,
			InternalMsg: ``,
			Code:        code,
			Data:        data,
		}
	}
}

func NewApi() *Api {
	return &Api{
		Strategies: []api_strategy2.StrategyData{},
	}
}

/**
wrap api处理器. 一个path一个，方法内分别处理method
*/
func WrapJson(methodController map[string]*Api) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		apiSession := api_session.NewApiSession() // 新建会话
		apiSession.ResponseWriter = response
		apiSession.Request = request
		apiSession.SetStatusCode(api_session.StatusCode_OK)
		// 应用层直接允许跨域。推荐接口层做跨域处理
		apiSession.SetHeader("Vary", "Origin, Access-Control-Request-Method, Access-Control-Request-Headers")
		apiSession.SetHeader("Access-Control-Allow-Origin", apiSession.GetHeader("Origin"))
		apiSession.SetHeader("Access-Control-Allow-Methods", apiSession.GetMethod())
		apiSession.SetHeader("Access-Control-Allow-Headers", "*")
		apiSession.SetHeader("Access-Control-Allow-Credentials", "true")
		requestMethod := apiSession.GetMethod()
		if requestMethod == string(api_session.ApiMethod_Option) {
			apiSession.WriteText(`ok`)
			return
		}
		var currentApi *Api
		if methodController[requestMethod] != nil { // 优先使用具体方法注册的控制器
			currentApi = methodController[requestMethod]
			apiSession.Api = currentApi
		} else if methodController[string(api_session.ApiMethod_All)] != nil {
			currentApi = methodController[string(api_session.ApiMethod_All)]
			apiSession.Api = currentApi
		} else {
			apiSession.WriteText(`not found`)
			return
		}

		defer go_error.Recover(func(msg string, internalMsg string, code uint64, data interface{}, err interface{}) {
			errMsg := fmt.Sprintf("msg: %s\ninternal_msg: %s", msg, internalMsg)
			logger.LoggerDriver.Logger.Error(
				"err: " +
					fmt.Sprint(err) +
					"\n" +
					errMsg +
					"\n" +
					apiSession.Datas[`error_msg`].(string) +
					"\n" +
					go_stack.Stack.GetStack(go_stack.Option{Skip: 0, Count: 30}))
			apiResult := DefaultReturnDataFunc(msg, internalMsg, code, data)
			if currentApi.ReturnHookFunc != nil {
				hookApiResult, err := currentApi.ReturnHookFunc(apiSession, apiResult)
				if err != nil {
					apiSession.WriteJson(DefaultReturnDataFunc(err.ErrorMessage, err.InternalErrorMessage, err.ErrorCode, err.Data))
					return
				}
				if hookApiResult == nil {
					return
				}
				apiSession.WriteJson(hookApiResult)
			} else {
				apiSession.WriteJson(apiResult)
			}
		})

		if !currentApi.IgnoreGlobalStrategies {
			for _, strategyData := range global_api_strategy.GlobalApiStrategyDriver.GlobalStrategies {
				if strategyData.Disable {
					continue
				}
				func() {
					defer go_error.Recover(func(msg string, internalMsg string, code uint64, data interface{}, err interface{}) {
						if code == go_error.INTERNAL_ERROR_CODE {
							code = strategyData.Strategy.GetErrorCode()
						}
						go_error.ThrowErrorWithDataInternalMsg(msg, internalMsg, code, data, err)
					})
					strategyData.Strategy.Execute(apiSession, strategyData.Param)
				}()
			}
		}

		for _, strategyData := range currentApi.Strategies {
			if strategyData.Disable {
				continue
			}
			func() {
				defer go_error.Recover(func(msg string, internalMsg string, code uint64, data interface{}, err interface{}) {
					if code == go_error.INTERNAL_ERROR_CODE {
						code = strategyData.Strategy.GetErrorCode()
					}
					go_error.ThrowErrorWithDataInternalMsg(msg, internalMsg, code, data, err)
				})
				strategyData.Strategy.Execute(apiSession, strategyData.Param)
			}()
		}
		for _, defer_ := range apiSession.Defers {
			defer defer_()
		}

		result := currentApi.Controller(apiSession)
		if result == nil {
			return
		}
		apiResult := DefaultReturnDataFunc(``, ``, 0, result)
		if currentApi.ReturnHookFunc != nil {
			hookApiResult, err := currentApi.ReturnHookFunc(apiSession, apiResult)
			if err != nil {
				apiSession.WriteJson(DefaultReturnDataFunc(err.ErrorMessage, err.InternalErrorMessage, err.ErrorCode, err.Data))
				return
			}
			if hookApiResult == nil {
				return
			}
			apiSession.WriteJson(hookApiResult)
		} else {
			apiSession.WriteJson(apiResult)
		}
	}
}
