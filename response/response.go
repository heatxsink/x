package response

import (
	"encoding/json"
	"net/http"
)

type Error struct {
	Message string `json:"message"`
}

type Response struct {
	IsError    bool        `json:"is_error"`
	Error      Error       `json:"error"`
	StatusCode int         `json:"status_code"`
	StatusText string      `json:"status_text"`
	Payload    interface{} `json:"payload"`
}

func Unauthorized(w http.ResponseWriter, err error) {
	resp := &Response{
		IsError: true,
		Error: Error{
			Message: err.Error(),
		},
		StatusCode: http.StatusUnauthorized,
		StatusText: http.StatusText(http.StatusUnauthorized),
	}
	JSON(w, http.StatusUnauthorized, &resp)
}

func BadRequest(w http.ResponseWriter, err error) {
	resp := &Response{
		IsError: true,
		Error: Error{
			Message: err.Error(),
		},
		StatusCode: http.StatusBadRequest,
		StatusText: http.StatusText(http.StatusBadRequest),
	}
	JSON(w, http.StatusBadRequest, &resp)
}

func OK(w http.ResponseWriter, payload interface{}) {
	resp := &Response{
		IsError:    false,
		StatusCode: http.StatusOK,
		StatusText: http.StatusText(http.StatusOK),
		Payload:    payload,
	}
	JSON(w, http.StatusOK, &resp)
}

func JSON(w http.ResponseWriter, httpStatus int, object interface{}) {
	httpBody, _ := json.MarshalIndent(&object, "", "  ")
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(httpStatus)
	w.Write(httpBody)
}
