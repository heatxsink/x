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

func JSONUnauthorized(w http.ResponseWriter, r *http.Request, err error) {
	resp := &Response{
		ErrorMessage: err.Error(),
		StatusCode:   http.StatusUnauthorized,
	}
	JSON(w, r, http.StatusUnauthorized, &resp)
}

func JSONOK(w http.ResponseWriter, r *http.Request, message string) {
	resp := &Response{
		Message:    message,
		StatusCode: http.StatusOK,
	}
	JSON(w, r, http.StatusOK, &resp)
}

func JSON(w http.ResponseWriter, r *http.Request, httpStatus int, object interface{}) {
	httpBody, _ := json.MarshalIndent(&object, "", "  ")
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(httpStatus)
	w.Write(httpBody)
}
