package service

import (
    "errors"
    "time"

    "github.com/google/uuid"
    "gorm.io/gorm"

    "sgin/model"
    "sgin/pkg/app"
)

type ReturnService struct{}

func NewReturnService() *ReturnService { return &ReturnService{} }

type ReqReturnItem struct {
    ProductItemID string `json:"product_item_id"`
    Quantity int `json:"quantity"`
}

// CreateReturnRequest 用户发起退货申请（支持部分退货）
func (s *ReturnService) CreateReturnRequest(ctx *app.Context, orderNo, userID, reason string, items []ReqReturnItem, amount float64) (*model.Return, error) {
    if ctx == nil || ctx.DB == nil {
        return nil, errors.New("invalid context")
    }

    // 验证订单存在
    var order model.Order
    if err := ctx.DB.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
        ctx.Logger.Error("order not found for return request", err)
        return nil, errors.New("order not found")
    }

    r := &model.Return{
        Uuid: uuid.New().String(),
        OrderNo: orderNo,
        UserID: userID,
        Reason: reason,
        Amount: amount,
        Status: string(model.ReturnStatusRequested),
        CreatedAt: time.Now().Format(time.DateTime),
        UpdatedAt: time.Now().Format(time.DateTime),
    }

    err := ctx.DB.Transaction(func(tx *gorm.DB) error {
        if err := tx.Create(r).Error; err != nil {
            return err
        }
        for _, it := range items {
            ri := &model.ReturnItem{
                Uuid: uuid.New().String(),
                ReturnUuid: r.Uuid,
                ProductItemID: it.ProductItemID,
                Quantity: it.Quantity,
                CreatedAt: time.Now().Format(time.DateTime),
                UpdatedAt: time.Now().Format(time.DateTime),
            }
            if err := tx.Create(ri).Error; err != nil {
                return err
            }
        }
        // 标记订单为申请退货（业务可选择是否改变订单主状态）
        if err := tx.Model(&model.Order{}).Where("order_no = ?", orderNo).Updates(map[string]interface{}{"status": model.OrderStatusReturnRequested, "updated_at": time.Now().Format(time.DateTime)}).Error; err != nil {
            return err
        }
        return nil
    })
    if err != nil {
        ctx.Logger.Error("failed to create return request", err)
        return nil, errors.New("failed to create return request")
    }
    return r, nil
}

// ApproveReturn 审核通过退货：回补库存并发起退款（如果可退）
func (s *ReturnService) ApproveReturn(ctx *app.Context, returnUuid string, refundAmount float64) error {
    if ctx == nil || ctx.DB == nil {
        return errors.New("invalid context")
    }

    return ctx.DB.Transaction(func(tx *gorm.DB) error {
        var r model.Return
        if err := tx.Where("uuid = ?", returnUuid).First(&r).Error; err != nil {
            return err
        }
        if r.Status != string(model.ReturnStatusRequested) {
            return errors.New("return request not in requested status")
        }

        // load return items
        var items []*model.ReturnItem
        if err := tx.Where("return_uuid = ?", r.Uuid).Find(&items).Error; err != nil {
            return err
        }

        // 回补库存
        for _, it := range items {
            if err := tx.Model(&model.ProductItem{}).Where("uuid = ?", it.ProductItemID).UpdateColumn("stock", gorm.Expr("stock + ?", int64(it.Quantity))).Error; err != nil {
                return err
            }
        }

        // 创建退款记录（若订单有已支付的 payment，则标记退款）
        var pay model.Payment
        payErr := tx.Where("order_id = ? AND status = ?", r.OrderNo, model.PaymentStatusPaid).First(&pay).Error
        if payErr == nil {
            // create refund record
            rf := &model.Refund{
                Uuid: uuid.New().String(),
                PaymentUuid: pay.Uuid,
                OrderID: r.OrderNo,
                Amount: refundAmount,
                Status: model.RefundStatusCompleted,
                CreatedAt: time.Now().Format(time.DateTime),
                UpdatedAt: time.Now().Format(time.DateTime),
            }
            if err := tx.Create(rf).Error; err != nil {
                return err
            }
            // mark payment refunded
            if err := tx.Model(&model.Payment{}).Where("uuid = ?", pay.Uuid).Updates(map[string]interface{}{"status": model.PaymentStatusRefunded, "updated_at": time.Now().Format(time.DateTime)}).Error; err != nil {
                return err
            }
        }

        // 更新退货记录状态
        if err := tx.Model(&model.Return{}).Where("uuid = ?", r.Uuid).Updates(map[string]interface{}{"status": string(model.ReturnStatusCompleted), "processed_at": time.Now().Format(time.DateTime), "updated_at": time.Now().Format(time.DateTime)}).Error; err != nil {
            return err
        }

        // 若需要，把订单状态标为已退款
        if err := tx.Model(&model.Order{}).Where("order_no = ?", r.OrderNo).Updates(map[string]interface{}{"status": model.OrderStatusRefunded, "updated_at": time.Now().Format(time.DateTime)}).Error; err != nil {
            return err
        }

        return nil
    })
}

// RejectReturn 拒绝退货申请
func (s *ReturnService) RejectReturn(ctx *app.Context, returnUuid string, reason string) error {
    if ctx == nil || ctx.DB == nil {
        return errors.New("invalid context")
    }
    if err := ctx.DB.Model(&model.Return{}).Where("uuid = ?", returnUuid).Updates(map[string]interface{}{"status": string(model.ReturnStatusRejected), "updated_at": time.Now().Format(time.DateTime)}).Error; err != nil {
        ctx.Logger.Error("failed to reject return", err)
        return errors.New("failed to reject return")
    }
    // 可记录拒绝原因到日志或扩展字段（此处简化）
    return nil
}
