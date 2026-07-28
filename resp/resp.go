package resp

import (
	"encoding/json"
	"net/http"
)

// ========== 业务错误码 ==========
// 0 表示成功，非 0 表示异常
const (
	CodeOK      = 0
	CodeErr     = 1 // 通用错误
	CodeAuthErr = 2 // 未登录或 token 过期
	CodeParamErr = 3 // 参数错误
	CodeNotFound = 4 // 资源不存在
)

var codeMsg = map[int]string{
	CodeOK:        "ok",
	CodeErr:       "error",
	CodeAuthErr:   "未登录或 token 已过期",
	CodeParamErr:  "参数错误",
	CodeNotFound:  "资源不存在",
}

// R 统一响应结构
//
//	前端拿到后先判断 Code:
//	  Code == 0 → 成功，取 Data
//	  Code != 0 → 失败，取 Message 提示用户
type R struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ========== 成功响应 ==========

// OK 返回成功（无数据）
func OK() *R {
	return &R{Code: CodeOK, Message: codeMsg[CodeOK]}
}

// Data 返回成功（带数据）
func Data(data interface{}) *R {
	return &R{Code: CodeOK, Message: codeMsg[CodeOK], Data: data}
}

// Page 返回分页列表
func Page(list interface{}, total int64) *R {
	return &R{
		Code:    CodeOK,
		Message: codeMsg[CodeOK],
		Data: map[string]interface{}{
			"list":  list,
			"total": total,
		},
	}
}

// ========== 失败响应 ==========

// Error 返回通用错误
func Error(msg string) *R {
	if msg == "" {
		msg = codeMsg[CodeErr]
	}
	return &R{Code: CodeErr, Message: msg}
}

// ErrCode 返回指定错误码
func ErrCode(code int, msg string) *R {
	if msg == "" {
		msg = codeMsg[code]
	}
	return &R{Code: code, Message: msg}
}

// ========== HTTP 输出 ==========

// Write 把 JSON 写入 http.ResponseWriter
//
//	func MyHandler(w http.ResponseWriter, r *http.Request) {
//	    resp.Write(w, resp.OK())
//	}
func Write(w http.ResponseWriter, r *R) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(r)
}
