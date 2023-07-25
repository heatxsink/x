package response

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	IsError    bool   `json:"is_error"`
	Message    string `json:"message"`
	StatusCode int    `json:"status_code"`
	StatusText string `json:"status_text"`
}

func Unauthorized(w http.ResponseWriter, err error) {
	resp := &Response{
		IsError:    true,
		Message:    err.Error(),
		StatusCode: http.StatusUnauthorized,
		StatusText: http.StatusText(http.StatusUnauthorized),
	}
	JSON(w, http.StatusUnauthorized, &resp)
}

func BadRequest(w http.ResponseWriter, err error) {
	resp := &Response{
		IsError:    true,
		Message:    err.Error(),
		StatusCode: http.StatusBadRequest,
		StatusText: http.StatusText(http.StatusBadRequest),
	}
	JSON(w, http.StatusBadRequest, &resp)
}

func OK(w http.ResponseWriter, message string) {
	resp := &Response{
		IsError:    false,
		Message:    message,
		StatusCode: http.StatusOK,
		StatusText: http.StatusText(http.StatusOK),
	}
	JSON(w, http.StatusOK, &resp)
}

func JSON(w http.ResponseWriter, httpStatus int, object interface{}) {
	httpBody, _ := json.MarshalIndent(&object, "", "  ")
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(httpStatus)
	w.Write(httpBody)
}
