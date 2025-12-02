package model

// 物流运单
type Shipment struct {
    ID int64 `json:"id" gorm:"primary_key"`
    Uuid string `json:"uuid" gorm:"type:varchar(36);unique_index"`
    OrderNo string `json:"order_no" gorm:"type:varchar(100);index"`
    Carrier string `json:"carrier" gorm:"type:varchar(100)"`
    TrackingNo string `json:"tracking_no" gorm:"type:varchar(200)"`
    Status string `json:"status" gorm:"type:varchar(50)"` // shipped, in_transit, delivered
    ShippedAt string `json:"shipped_at"`
    DeliveredAt string `json:"delivered_at"`
    CreatedAt string `gorm:"autoCreateTime;type:datetime" json:"created_at"`
    UpdatedAt string `gorm:"autoUpdateTime;type:datetime" json:"updated_at"`
}
