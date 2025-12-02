package service

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"sgin/model"
	"sgin/pkg/app"
	"gorm.io/gorm"
)

type ShipmentService struct{}

func NewShipmentService() *ShipmentService { return &ShipmentService{} }

// CreateShipment 创建运单并把订单状态标记为已发运
func (s *ShipmentService) CreateShipment(ctx *app.Context, orderNo, carrier, trackingNo string) (*model.Shipment, error) {
	// 检查订单存在
	var order model.Order
	if err := ctx.DB.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		ctx.Logger.Error("order not found for shipment", err)
		return nil, errors.New("order not found")
	}

	sh := &model.Shipment{
		Uuid:       uuid.New().String(),
		OrderNo:    orderNo,
		Carrier:    carrier,
		TrackingNo: trackingNo,
		Status:     "shipped",
		ShippedAt:  time.Now().Format(time.DateTime),
		CreatedAt:  time.Now().Format(time.DateTime),
		UpdatedAt:  time.Now().Format(time.DateTime),
	}

	err := ctx.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(sh).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Order{}).Where("order_no = ?", orderNo).Updates(map[string]interface{}{"status": model.OrderStatusShipped, "updated_at": time.Now().Format(time.DateTime)}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		ctx.Logger.Error("failed to create shipment", err)
		return nil, errors.New("failed to create shipment")
	}
	return sh, nil
}

// UpdateShipmentStatus 更新运单状态（如快递回调），并在必要时标记已送达
func (s *ShipmentService) UpdateShipmentStatus(ctx *app.Context, trackingNo, status string) error {
	var sh model.Shipment
	if err := ctx.DB.Where("tracking_no = ?", trackingNo).First(&sh).Error; err != nil {
		ctx.Logger.Error("shipment not found", err)
		return errors.New("shipment not found")
	}

	err := ctx.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Shipment{}).Where("uuid = ?", sh.Uuid).Updates(map[string]interface{}{"status": status, "updated_at": time.Now().Format(time.DateTime)}).Error; err != nil {
			return err
		}
		if status == "delivered" {
			// 标记订单已发货/已送达
			if err := tx.Model(&model.Order{}).Where("order_no = ?", sh.OrderNo).Updates(map[string]interface{}{"status": model.OrderStatusDelivered, "delivered_at": time.Now().Format(time.DateTime), "updated_at": time.Now().Format(time.DateTime)}).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		ctx.Logger.Error("failed to update shipment status", err)
		return errors.New("failed to update shipment status")
	}
	return nil
}
