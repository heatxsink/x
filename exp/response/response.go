package response

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	IsError      bool        `json:"is_error,omitempty"`
	ErrorMessage string      `json:"error_message,omitempty"`
	StatusCode   int         `json:"status_code,omitempty"`
	StatusText   string      `json:"status_text,omitempty"`
	Data         interface{} `json:"data,omitempty"`
}

func Unauthorized(w http.ResponseWriter, err error) {
	resp := &Response{
		IsError:      true,
		ErrorMessage: err.Error(),
		StatusCode:   http.StatusUnauthorized,
		StatusText:   http.StatusText(http.StatusUnauthorized),
	}
	JSON(w, http.StatusUnauthorized, &resp)
}

func BadRequest(w http.ResponseWriter, err error) {
	resp := &Response{
		IsError:      true,
		ErrorMessage: err.Error(),
		StatusCode:   http.StatusBadRequest,
		StatusText:   http.StatusText(http.StatusBadRequest),
	}
	JSON(w, http.StatusBadRequest, &resp)
}

func OK(w http.ResponseWriter, object interface{}) {
	resp := &Response{
		StatusCode: http.StatusOK,
		StatusText: http.StatusText(http.StatusOK),
		Data:       object,
	}
	JSON(w, http.StatusOK, &resp)
}

func JSONIndent(w http.ResponseWriter, httpStatus int, object interface{}) {
	httpBody, _ := json.MarshalIndent(&object, "", "  ")
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(httpStatus)
	w.Write(httpBody)
}

func JSON(w http.ResponseWriter, httpStatus int, object interface{}) {
	httpBody, _ := json.Marshal(&object)
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(httpStatus)
	w.Write(httpBody)
}
