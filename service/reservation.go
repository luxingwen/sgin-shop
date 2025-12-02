package service

import (
	"time"

	"sgin/model"
	"sgin/pkg/app"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ReservationService struct{}

func NewReservationService() *ReservationService {
	return &ReservationService{}
}

// ReleaseExpiredReservations 查找超时的预占订单，回滚库存并关闭订单
func (s *ReservationService) ReleaseExpiredReservations(ctx *app.Context) error {
	if ctx == nil || ctx.DB == nil {
		return nil
	}

	var expiredOrders []*model.Order
	now := time.Now().Format(time.DateTime)
	if err := ctx.DB.Where("status = ? AND reservation_expires_at <= ?", model.OrderStatusReserved, now).Find(&expiredOrders).Error; err != nil {
		ctx.Logger.Error("failed to query expired reservations", err)
		return err
	}

	for _, ord := range expiredOrders {
		err := ctx.DB.Transaction(func(tx *gorm.DB) error {
			// get order items
			items := make([]*model.OrderItem, 0)
			if err := tx.Where("order_id = ?", ord.OrderNo).Find(&items).Error; err != nil {
				return err
			}

			// return stock for each item (use UpdateColumn to avoid triggers)
			for _, it := range items {
				res := tx.Model(&model.ProductItem{}).Where("uuid = ?", it.ProductItemID).
					UpdateColumn("stock", clause.Expr{SQL: "stock + ?", Vars: []interface{}{int64(it.Quantity)}})
				if res.Error != nil {
					return res.Error
				}
			}

			// mark order closed
			if err := tx.Model(&model.Order{}).Where("order_no = ?", ord.OrderNo).Updates(map[string]interface{}{
				"status":     model.OrderStatusClosed,
				"closed_at":  time.Now().Format(time.DateTime),
				"updated_at": time.Now().Format(time.DateTime),
			}).Error; err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			ctx.Logger.Error("failed to release reservation for order", err)
		}
	}

	return nil
}

// StartReservationReleaser 启动周期任务，每 interval 检查并释放过期预占
func (s *ReservationService) StartReservationReleaser(ctx *app.Context, interval time.Duration) {
	if ctx == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			_ = s.ReleaseExpiredReservations(ctx)
		}
	}()
}

// 兼容导出函数（保持向后兼容）
func ReleaseExpiredReservations(ctx *app.Context) error {
	return NewReservationService().ReleaseExpiredReservations(ctx)
}

func StartReservationReleaser(ctx *app.Context, interval time.Duration) {
	NewReservationService().StartReservationReleaser(ctx, interval)
}
