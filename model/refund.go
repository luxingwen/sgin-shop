package model

type RefundStatus string

const (
	RefundStatusRequested  = "requested"
	RefundStatusProcessing = "processing"
	RefundStatusCompleted  = "completed"
	RefundStatusFailed     = "failed"
)

type Refund struct {
	ID   string `json:"id" gorm:"primary_key"`
	Uuid string `json:"uuid" gorm:"type:varchar(36);unique_index"`
	// 关联 payment
	PaymentUuid     string  `json:"payment_uuid" gorm:"type:varchar(36);index"`
	OrderID         string  `json:"order_id" gorm:"index"`
	Amount          float64 `json:"amount"`
	Status          string  `json:"status"`
	Reason          string  `json:"reason"`
	ChannelRefundNo string  `json:"channel_refund_no" gorm:"type:varchar(100)"`
	CreatedAt       string  `gorm:"autoCreateTime;type:datetime" json:"created_at"`
	UpdatedAt       string  `gorm:"autoUpdateTime;type:datetime" json:"updated_at"`
}
