package service

import (
    "bytes"
    "encoding/csv"
    "fmt"
    "time"

    "sgin/model"
    "sgin/pkg/app"
)

// PaymentReconciler 提供对账与导出功能
type PaymentReconciler struct{}

func NewPaymentReconciler() *PaymentReconciler { return &PaymentReconciler{} }

type ReconcileQuery struct {
    Page        int    `form:"page" json:"page"`
    PageSize    int    `form:"page_size" json:"page_size"`
    Channel     string `form:"channel" json:"channel"`
    Method      string `form:"method" json:"method"`
    ChannelStatus string `form:"channel_status" json:"channel_status"`
    From        string `form:"from" json:"from"` // RFC3339 或 yyyy-mm-dd
    To          string `form:"to" json:"to"`
}

// ListUnmatchedPayments 返回未匹配到订单或订单状态与支付状态不一致的支付记录，支持分页/筛选/时间范围
func (r *PaymentReconciler) ListUnmatchedPayments(ctx *app.Context, q ReconcileQuery) ([]*model.Payment, int64, error) {
    if q.Page <= 0 {
        q.Page = 1
    }
    if q.PageSize <= 0 || q.PageSize > 1000 {
        q.PageSize = 50
    }

    db := ctx.DB.Table("payments p").Select("p.*").Joins("left join orders o on p.order_id = o.order_no").Where("p.status = ?", model.PaymentStatusPaid)

    // only include payments that are either not associated to order, or order status != paid
    db = db.Where("(p.order_id = '' OR o.order_no IS NULL OR o.status <> ?)", model.OrderStatusPaid)

    if q.Channel != "" {
        db = db.Where("p.channel = ?", q.Channel)
    }
    if q.Method != "" {
        db = db.Where("p.method = ?", q.Method)
    }
    if q.ChannelStatus != "" {
        db = db.Where("p.channel_status = ?", q.ChannelStatus)
    }

    // 时间范围过滤，使用 payments.created_at 字段
    if q.From != "" {
        if t, err := time.Parse(time.RFC3339, q.From); err == nil {
            db = db.Where("p.created_at >= ?", t.Format(time.DateTime))
        } else if t2, err2 := time.Parse("2006-01-02", q.From); err2 == nil {
            db = db.Where("p.created_at >= ?", t2.Format(time.DateTime))
        }
    }
    if q.To != "" {
        if t, err := time.Parse(time.RFC3339, q.To); err == nil {
            db = db.Where("p.created_at <= ?", t.Format(time.DateTime))
        } else if t2, err2 := time.Parse("2006-01-02", q.To); err2 == nil {
            // include whole day
            t2 = t2.Add(24*time.Hour - time.Nanosecond)
            db = db.Where("p.created_at <= ?", t2.Format(time.DateTime))
        }
    }

    var total int64
    if err := db.Count(&total).Error; err != nil {
        return nil, 0, err
    }

    var payments []*model.Payment
    if err := db.Order("p.created_at desc").Limit(q.PageSize).Offset((q.Page-1)*q.PageSize).Find(&payments).Error; err != nil {
        return nil, 0, err
    }
    return payments, total, nil
}

// ExportUnmatchedPaymentsCSV 导出未匹配支付为 CSV
func (r *PaymentReconciler) ExportUnmatchedPaymentsCSV(ctx *app.Context, q ReconcileQuery) ([]byte, error) {
    payments, _, err := r.ListUnmatchedPayments(ctx, q)
    if err != nil {
        return nil, err
    }
    buf := &bytes.Buffer{}
    w := csv.NewWriter(buf)
    // header
    _ = w.Write([]string{"uuid", "order_id", "amount", "status", "channel", "channel_txn", "paid_at"})
    for _, p := range payments {
        _ = w.Write([]string{p.Uuid, p.OrderID, fmt.Sprintf("%.2f", p.Amount), p.Status, p.Channel, p.ChannelTransactionNo, p.PaidAt})
    }
    w.Flush()
    return buf.Bytes(), nil
}

// RunReconciliation 执行一次对账：将无需人工干预的异常进行标记或尝试补偿（此处仅标记为需要人工复核）
func (r *PaymentReconciler) RunReconciliation(ctx *app.Context) (int, error) {
    payments, _, err := r.ListUnmatchedPayments(ctx, ReconcileQuery{Page:1, PageSize:1000})
    if err != nil {
        return 0, err
    }
    now := time.Now().Format(time.DateTime)
    count := 0
    for _, p := range payments {
        // 标记为 channel_status = "needs_reconcile" 并记录更新时间
        if err := ctx.DB.Model(&model.Payment{}).Where("uuid = ?", p.Uuid).Updates(map[string]interface{}{"channel_status": "needs_reconcile", "updated_at": now}).Error; err != nil {
            return count, err
        }
        count++
    }
    return count, nil
}

// ManualMarkReconciled 手动标记支付为已对账（例如人工确认后）
func (r *PaymentReconciler) ManualMarkReconciled(ctx *app.Context, paymentUuid string) error {
    return ctx.DB.Model(&model.Payment{}).Where("uuid = ?", paymentUuid).Updates(map[string]interface{}{"channel_status": "reconciled", "updated_at": time.Now().Format(time.DateTime)}).Error
}
