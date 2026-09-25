package adapters

// Response is used for static shape json return
type Response struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Error   any    `json:"error"`
	Data    any    `json:"data"`
}

// BuildResponse method is to inject data value to dynamic success response
func BuildResponse(message string, data any, err any) Response {
	var errmess any = nil
	if err != nil {
		errmess = err
	}
	res := Response{
		Status:  true,
		Message: message,
		Error:   errmess,
		Data:    data,
	}
	return res
}

// BuildErrorResponse method is to inject data value to dynamic failed response
func BuildErrorResponse(message string, data any, err error) Response {
	res := Response{
		Status:  false,
		Message: message,
		Error:   err.Error(),
		Data:    data,
	}
	return res
}
