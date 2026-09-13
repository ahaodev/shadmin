package domain

const (
	DefaultPage     = 1
	DefaultPageSize = 10
	MaxPageSize     = 10000
)

// QueryParams 通用查询参数，包含分页和排序信息。
type QueryParams struct {
	Page     int    `json:"page" form:"page"`
	PageSize int    `json:"page_size" form:"page_size"`
	SortBy   string `json:"sort_by" form:"sort_by"`
	Order    string `json:"order" form:"order"` // asc, desc
}

func (qp *QueryParams) Paginate() (offset, limit int) {
	if qp.Page < DefaultPage {
		qp.Page = DefaultPage
	}
	if qp.PageSize <= 0 {
		qp.PageSize = DefaultPageSize
	}
	if qp.PageSize > MaxPageSize {
		qp.PageSize = MaxPageSize
	}
	return (qp.Page - 1) * qp.PageSize, qp.PageSize
}
