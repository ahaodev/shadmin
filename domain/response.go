package domain

const (
	SuccessCode = 0
	FailCode    = 1
)

type Response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

func RespError(msg any) Response {
	var errMsg string
	switch v := msg.(type) {
	case string:
		errMsg = v
	case error:
		errMsg = v.Error()
	default:
		errMsg = "Unknown error"
	}
	return Response{Code: FailCode, Msg: errMsg, Data: nil}
}

func RespSuccess(data any) Response {
	return Response{Code: SuccessCode, Msg: "OK", Data: data}
}

type PagedResult[T any] struct {
	List       []T `json:"list"`
	Total      int `json:"total"`
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalPages int `json:"total_pages"`
}

func NewPagedResult[T any](data []T, total, page, pageSize int) *PagedResult[T] {
	if page < DefaultPage {
		page = DefaultPage
	}
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	totalPages := (total + pageSize - 1) / pageSize
	// 空列表序列化为 []。
	if data == nil {
		data = []T{}
	}
	return &PagedResult[T]{
		List:       data,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}
}
