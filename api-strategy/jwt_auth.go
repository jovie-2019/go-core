package api_strategy

import (
	jwt2 "github.com/dgrijalva/jwt-go"
	"github.com/pefish/go-core/api-session"
	"github.com/pefish/go-core/driver/logger"
	"github.com/pefish/go-core/util"
	"github.com/pefish/go-error"
	"github.com/pefish/go-jwt"
	"github.com/pefish/go-reflect"
)

type JwtAuthStrategyClass struct {
	errorCode           uint64
	pubKey              string
	headerName          string
	noCheckExpire       bool
	disableUserId       bool
	errorMsg            string
}

var JwtAuthApiStrategy = JwtAuthStrategyClass{
	errorCode:           go_error.INTERNAL_ERROR_CODE,
	errorMsg:            `Unauthorized`,
}

type JwtAuthParam struct {
}

func (this *JwtAuthStrategyClass) GetName() string {
	return `jwtAuth`
}

func (this *JwtAuthStrategyClass) GetDescription() string {
	return `jwt auth`
}

func (this *JwtAuthStrategyClass) SetErrorCode(code uint64) {
	this.errorCode = code
}

func (this *JwtAuthStrategyClass) SetErrorMessage(msg string) {
	this.errorMsg = msg
}

func (this *JwtAuthStrategyClass) GetErrorCode() uint64 {
	return this.errorCode
}

func (this *JwtAuthStrategyClass) SetNoCheckExpire() {
	this.noCheckExpire = true
}

func (this *JwtAuthStrategyClass) DisableUserId() {
	this.disableUserId = true
}

func (this *JwtAuthStrategyClass) SetPubKey(pubKey string) {
	this.pubKey = pubKey
}

func (this *JwtAuthStrategyClass) SetHeaderName(headerName string) {
	this.headerName = headerName
}

func (this *JwtAuthStrategyClass) Execute(out *api_session.ApiSessionClass, param interface{}) {
	logger.LoggerDriver.Logger.DebugF(`api-strategy %s trigger`, this.GetName())
	out.JwtHeaderName = this.headerName
	jwt := out.GetHeader(this.headerName)

	verifyResult, token, err := go_jwt.Jwt.VerifyJwt(this.pubKey, jwt, this.noCheckExpire)
	if err != nil {
		go_error.ThrowWithInternalMsg(this.errorMsg, err.Error(), this.errorCode)
	}
	if !verifyResult {
		go_error.ThrowWithInternalMsg(this.errorMsg, `jwt verify error or jwt expired`, this.errorCode)
	}
	out.JwtBody = token.Claims.(jwt2.MapClaims)
	if !this.disableUserId {
		jwtPayload := out.JwtBody[`payload`].(map[string]interface{})
		if jwtPayload[`user_id`] == nil {
			go_error.ThrowWithInternalMsg(this.errorMsg, `jwt verify error, user_id not exist`, this.errorCode)
		}

		userId := go_reflect.Reflect.MustToUint64(jwtPayload[`user_id`])
		out.UserId = userId

		util.UpdateSessionErrorMsg(out, `jwtAuth`, userId)
	}
}
