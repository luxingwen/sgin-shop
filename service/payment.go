package service

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"sgin/model"
	"sgin/pkg/app"

	"github.com/google/uuid"
)

type PaymentService struct {
}

func NewPaymentService() *PaymentService {
	return &PaymentService{}
}

// CreatePayment 创建一个新的付款记录
func (s *PaymentService) CreatePayment(ctx *app.Context, payment *model.Payment) (*model.Payment, error) {
	payment.Uuid = uuid.New().String()
	payment.CreatedAt = time.Now().Format(time.DateTime)
	payment.UpdatedAt = payment.CreatedAt
	err := ctx.DB.Create(&payment).Error
	if err != nil {
		ctx.Logger.Error("Failed to create payment", err)
		return nil, errors.New("failed to create payment")
	}

	return payment, nil
}

// GetPaymentByUUID 根据UUID获取付款记录
func (s *PaymentService) GetPaymentByUUID(ctx *app.Context, uuid string) (*model.Payment, error) {
	var payment model.Payment
	err := ctx.DB.Where("uuid = ?", uuid).First(&payment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("payment not found")
		}
		ctx.Logger.Error("Failed to get payment by UUID", err)
		return nil, errors.New("failed to get payment by UUID")
	}
	return &payment, nil
}

// UpdatePayment 更新付款记录
func (s *PaymentService) UpdatePayment(ctx *app.Context, payment *model.Payment) error {
	now := time.Now()
	payment.UpdatedAt = now.Format(time.DateTime)
	err := ctx.DB.Updates(payment).Error
	if err != nil {
		ctx.Logger.Error("Failed to update payment", err)
		return errors.New("failed to update payment")
	}

	return nil
}

// DeletePayment 删除付款记录
func (s *PaymentService) DeletePayment(ctx *app.Context, uuid string) error {
	err := ctx.DB.Where("uuid = ?", uuid).Delete(&model.Payment{}).Error
	if err != nil {
		ctx.Logger.Error("Failed to delete payment", err)
		return errors.New("failed to delete payment")
	}

	return nil
}

// GetPaymentList 获取付款记录列表
func (s *PaymentService) GetPaymentList(ctx *app.Context, params *model.ReqPaymentQueryParam) (*model.PagedResponse, error) {
	var (
		payments []*model.Payment
		total    int64
	)
	db := ctx.DB.Model(&model.Payment{})
	// 应用UserID和OrderID过滤条件

	if params.UserID != "" {
		db = db.Where("user_id = ?", params.UserID)
	}
	if params.OrderID != "" {
		db = db.Where("order_id = ?", params.OrderID)
	}

	err := db.Count(&total).Error
	if err != nil {
		ctx.Logger.Error("Failed to get payment count", err)
		return nil, errors.New("failed to get payment count")
	}

	err = db.Order("id DESC").Offset(params.GetOffset()).Limit(params.PageSize).Find(&payments).Error
	if err != nil {
		ctx.Logger.Error("Failed to get payment list", err)
		return nil, errors.New("failed to get payment list")
	}

	return &model.PagedResponse{
		Total: total,
		Data:  payments,
	}, nil
}

// HandleNotification 处理支付渠道的异步通知，保证幂等性并在事务中更新付款与订单状态
func (s *PaymentService) HandleNotification(ctx *app.Context, channel string, channelOrderNo string, channelTransactionNo string, amount float64, channelStatus string, rawData string) error {
	// 尝试查找已有的 payment
	var payment model.Payment
	err := ctx.DB.Where("channel_transaction_no = ? OR channel_order_no = ? OR order_id = ?", channelTransactionNo, channelOrderNo, channelOrderNo).First(&payment).Error
	if err == nil {
		// 已存在记录
		if payment.Status == model.PaymentStatusPaid {
			// 幂等：已处理
			return nil
		}
	} else {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.Logger.Error("Failed to query payment for notification", err)
			return err
		}
	}

	// 查找订单
	var order model.Order
	if channelOrderNo == "" {
		ctx.Logger.Error("notification missing order id")
		return errors.New("missing order id in notification")
	}
	if err := ctx.DB.Where("order_no = ?", channelOrderNo).First(&order).Error; err != nil {
		ctx.Logger.Error("Order not found for notification", err)
		return err
	}

	// 简单金额校验：若通知方提供了金额且订单总金额已存在，则比对两者差异
	if amount > 0 && order.TotalAmount > 0 {
		// 允许小幅浮点误差
		diff := amount - order.TotalAmount
		if diff < 0 {
			diff = -diff
		}
		if diff > 0.01 {
			ctx.Logger.Error("notification amount mismatch", "order_no", order.OrderNo, "order_amount", order.TotalAmount, "notify_amount", amount)
			return errors.New("amount mismatch between order and notification")
		}
	}

	// 在事务中更新或创建 payment，并更新订单状态
	now := time.Now().Format(time.DateTime)
	txErr := ctx.DB.Transaction(func(tx *gorm.DB) error {
		// refresh payment lookup within tx
		var p model.Payment
		qerr := tx.Where("channel_transaction_no = ? OR channel_order_no = ? OR order_id = ?", channelTransactionNo, channelOrderNo, channelOrderNo).First(&p).Error
		if qerr != nil {
			if qerr == gorm.ErrRecordNotFound {
				// create payment record
				p = model.Payment{
					Uuid:                 uuid.New().String(),
					UserID:               order.UserID,
					OrderID:              order.OrderNo,
					Amount:               amount,
					Status:               model.PaymentStatusPaid,
					Method:               channel,
					Channel:              channel,
					ChannelOrderNo:       channelOrderNo,
					ChannelTransactionNo: channelTransactionNo,
					ChannelStatus:        channelStatus,
					ChannelData:          rawData,
					PaidAt:               now,
					CreatedAt:            now,
					UpdatedAt:            now,
				}
				if cerr := tx.Create(&p).Error; cerr != nil {
					ctx.Logger.Error("Failed to create payment in notification tx", cerr)
					return cerr
				}
			} else {
				ctx.Logger.Error("Failed to query payment in tx", qerr)
				return qerr
			}
		} else {
			// update existing payment
			if p.Status == model.PaymentStatusPaid {
				return nil
			}
			p.ChannelTransactionNo = channelTransactionNo
			p.ChannelOrderNo = channelOrderNo
			p.ChannelStatus = channelStatus
			p.ChannelData = rawData
			p.PaidAt = now
			p.Status = model.PaymentStatusPaid
			p.UpdatedAt = now
			if uerr := tx.Save(&p).Error; uerr != nil {
				ctx.Logger.Error("Failed to update payment in tx", uerr)
				return uerr
			}
		}

		// 更新订单状态为已支付（如果尚未标记为已支付）
		if order.Status == model.OrderStatusClosed {
			// 已关闭的订单不应该被支付，记录并返回错误，交由上层处理（例如触发退款或人工介入）
			ctx.Logger.Error("notification received for closed order", "order_no", order.OrderNo)
			return errors.New("order already closed")
		}

		if order.Status != model.OrderStatusPaid {
			updates := map[string]interface{}{"status": model.OrderStatusPaid, "paid_at": now, "updated_at": now}
			// 清理预占字段
			if order.Status == model.OrderStatusReserved {
				updates["reserved_at"] = ""
				updates["reservation_expires_at"] = ""
			}
			if uerr := tx.Model(&model.Order{}).Where("order_no = ?", order.OrderNo).Updates(updates).Error; uerr != nil {
				ctx.Logger.Error("Failed to update order status in tx", uerr)
				return uerr
			}
		}

		return nil
	})

	if txErr != nil {
		return txErr
	}

	return nil
}
