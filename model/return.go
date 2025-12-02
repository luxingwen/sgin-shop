package model

// 退货/售后相关模型
type ReturnStatus string

const (
    ReturnStatusRequested = "requested"
    ReturnStatusApproved  = "approved"
    ReturnStatusRejected  = "rejected"
    ReturnStatusCompleted = "completed"
)

type Return struct {
    ID        int64  `json:"id" gorm:"primary_key"`
    Uuid      string `json:"uuid" gorm:"type:varchar(36);unique_index"`
    OrderNo   string `json:"order_no" gorm:"type:varchar(100);index"`
    UserID    string `json:"user_id" gorm:"type:varchar(100);index"`
    Reason    string `json:"reason" gorm:"type:text"`
    Amount    float64 `json:"amount"` // 申请退款金额（可部分退款）
    Status    string `json:"status" gorm:"type:varchar(50)"`
    ProcessedAt string `json:"processed_at"`
    CreatedAt string `gorm:"autoCreateTime;type:datetime" json:"created_at"`
    UpdatedAt string `gorm:"autoUpdateTime;type:datetime" json:"updated_at"`
}

// ReturnItem 记录退货的具体商品与数量
type ReturnItem struct {
    ID int64 `json:"id" gorm:"primary_key"`
    Uuid string `json:"uuid" gorm:"type:varchar(36);unique_index"`
    ReturnUuid string `json:"return_uuid" gorm:"type:varchar(36);index"`
    ProductItemID string `json:"product_item_id" gorm:"type:varchar(36);index"`
    Quantity int `json:"quantity"`
    CreatedAt string `gorm:"autoCreateTime;type:datetime" json:"created_at"`
    UpdatedAt string `gorm:"autoUpdateTime;type:datetime" json:"updated_at"`
}
