package dto

type APIResponse struct {
	Error    *ErrorResponse `json:"error,omitempty"`
	Response any            `json:"response,omitempty"`
	Data     any            `json:"data,omitempty"`
}

type ErrorResponse struct {
	Code int    `json:"code"`
	Text string `json:"text"`
}
