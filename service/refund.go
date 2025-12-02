package service

import (
	"errors"
	"time"

	"sgin/model"
	"sgin/pkg/app"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RefundService struct{}

func NewRefundService() *RefundService { return &RefundService{} }

// CreateRefund 创建退款申请（仅记录，实际退款由支付渠道/人工处理）
func (s *RefundService) CreateRefund(ctx *app.Context, paymentUuid string, amount float64, reason string) (*model.Refund, error) {
	// 查 payment
	var p model.Payment
	if err := ctx.DB.Where("uuid = ?", paymentUuid).First(&p).Error; err != nil {
		ctx.Logger.Error("payment not found for refund", err)
		return nil, errors.New("payment not found")
	}

	r := &model.Refund{
		Uuid:        uuid.New().String(),
		PaymentUuid: p.Uuid,
		OrderID:     p.OrderID,
		Amount:      amount,
		Status:      model.RefundStatusRequested,
		Reason:      reason,
		CreatedAt:   time.Now().Format(time.DateTime),
		UpdatedAt:   time.Now().Format(time.DateTime),
	}
	if err := ctx.DB.Create(r).Error; err != nil {
		ctx.Logger.Error("failed to create refund record", err)
		return nil, errors.New("failed to create refund")
	}
	return r, nil
}

// ProcessRefund 标记退款为完成（通常在第三方退款成功回调或人工确认后调用）
func (s *RefundService) ProcessRefund(ctx *app.Context, refundUuid string) error {
	var r model.Refund
	if err := ctx.DB.Where("uuid = ?", refundUuid).First(&r).Error; err != nil {
		ctx.Logger.Error("refund not found", err)
		return errors.New("refund not found")
	}

	// 在事务中更新退款、付款、订单并回补库存
	return ctx.DB.Transaction(func(tx *gorm.DB) error {
		// update refund
		if err := tx.Model(&model.Refund{}).Where("uuid = ?", r.Uuid).Updates(map[string]interface{}{"status": model.RefundStatusCompleted, "updated_at": time.Now().Format(time.DateTime)}).Error; err != nil {
			return err
		}

		// mark payment refunded
		if err := tx.Model(&model.Payment{}).Where("uuid = ?", r.PaymentUuid).Updates(map[string]interface{}{"status": model.PaymentStatusRefunded, "updated_at": time.Now().Format(time.DateTime)}).Error; err != nil {
			return err
		}

		// load order items and return stock
		var items []*model.OrderItem
		if err := tx.Where("order_id = ?", r.OrderID).Find(&items).Error; err != nil {
			return err
		}
		for _, it := range items {
			if err := tx.Model(&model.ProductItem{}).Where("uuid = ?", it.ProductItemID).UpdateColumn("stock", gorm.Expr("stock + ?", int64(it.Quantity))).Error; err != nil {
				return err
			}
		}

		// mark order refunded
		if err := tx.Model(&model.Order{}).Where("order_no = ?", r.OrderID).Updates(map[string]interface{}{"status": model.OrderStatusRefunded, "updated_at": time.Now().Format(time.DateTime)}).Error; err != nil {
			return err
		}

		return nil
	})
}
