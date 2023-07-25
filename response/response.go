package response

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	ErrorMessage string `json:"error_message"`
	Message      string `json:"message"`
	StatusCode   int    `json:"status_code"`
}

func Unauthorized(w http.ResponseWriter, err error) {
	resp := &Response{
		ErrorMessage: err.Error(),
		StatusCode:   http.StatusUnauthorized,
	}
	JSON(w, http.StatusUnauthorized, &resp)
}

func BadRequest(w http.ResponseWriter, err error) {
	resp := &Response{
		ErrorMessage: err.Error(),
		StatusCode:   http.StatusBadRequest,
	}
	JSON(w, http.StatusBadRequest, &resp)
}

func OK(w http.ResponseWriter, message string) {
	resp := &Response{
		Message:    message,
		StatusCode: http.StatusOK,
	}
	JSON(w, http.StatusOK, &resp)
}

func JSON(w http.ResponseWriter, httpStatus int, object interface{}) {
	httpBody, _ := json.MarshalIndent(&object, "", "  ")
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(httpStatus)
	w.Write(httpBody)
}
