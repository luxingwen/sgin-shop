package controller

import (
    "net/http"

    "sgin/pkg/app"
    "sgin/service"
    "sgin/model"
)

// PaymentReconcileController 管理端对账控制器
type PaymentReconcileController struct {
    Reconciler *service.PaymentReconciler
}

// RunReconcile 触发一次对账
// @Summary 触发对账作业
// @Tags 对账
// @Accept json
// @Produce json
// @Success 200 {object} app.Response
// @Router /api/v1/admin/payments/reconcile/run [post]
func (pc *PaymentReconcileController) RunReconcile(c *app.Context) {
    // 可接收可选分页/筛选参数
    q := service.ReconcileQuery{}
    _ = c.ShouldBindQuery(&q)
    n, err := pc.Reconciler.RunReconciliation(c)
    if err != nil {
        c.JSONError(http.StatusInternalServerError, err.Error())
        return
    }
    c.JSONSuccess(map[string]interface{}{"marked": n})
}

// ListUnmatched 列出未匹配支付，支持分页/筛选/时间范围
// @Summary 列出未匹配支付
// @Tags 对账
// @Accept json
// @Produce json
// @Param page query int false "page"
// @Param page_size query int false "page_size"
// @Param channel query string false "channel"
// @Param method query string false "method"
// @Param channel_status query string false "channel_status"
// @Param from query string false "from" Format("date-time")
// @Param to query string false "to" Format("date-time")
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/admin/payments/reconcile/unmatched [get]
func (pc *PaymentReconcileController) ListUnmatched(c *app.Context) {
    // bind generic pagination first
    pag := model.Pagination{}
    if err := c.ShouldBindQuery(&pag); err != nil {
        c.JSONError(http.StatusBadRequest, err.Error())
        return
    }
    // bind additional reconcile filters (channel/method/channel_status/from/to)
    q := service.ReconcileQuery{}
    _ = c.ShouldBindQuery(&q)
    // map pagination fields
    if pag.Current > 0 {
        q.Page = pag.Current
    }
    if pag.PageSize > 0 {
        q.PageSize = pag.PageSize
    }
    if pag.StartTime != "" {
        q.From = pag.StartTime
    }
    if pag.EndTime != "" {
        q.To = pag.EndTime
    }

    ps, total, err := pc.Reconciler.ListUnmatchedPayments(c, q)
    if err != nil {
        c.JSONError(http.StatusInternalServerError, err.Error())
        return
    }
    // build pagination result using shared schema
    page := q.Page
    if page <= 0 {
        page = 1
    }
    pageSize := q.PageSize
    if pageSize <= 0 {
        pageSize = 50
    }
    pr := &app.PaginationResult{Total: total, Current: page, PageSize: pageSize}
    c.ResPage(ps, pr)
}

// ExportUnmatched 导出 CSV（支持相同查询参数）
// @Summary 导出未匹配支付 CSV
// @Tags 对账
// @Produce text/csv
// @Router /api/v1/admin/payments/reconcile/export [get]
func (pc *PaymentReconcileController) ExportUnmatched(c *app.Context) {
    // bind generic pagination first
    pag := model.Pagination{}
    if err := c.ShouldBindQuery(&pag); err != nil {
        c.JSONError(http.StatusBadRequest, err.Error())
        return
    }
    // bind additional reconcile filters
    q := service.ReconcileQuery{}
    _ = c.ShouldBindQuery(&q)
    if pag.Current > 0 {
        q.Page = pag.Current
    }
    if pag.PageSize > 0 {
        q.PageSize = pag.PageSize
    }
    if pag.StartTime != "" {
        q.From = pag.StartTime
    }
    if pag.EndTime != "" {
        q.To = pag.EndTime
    }

    b, err := pc.Reconciler.ExportUnmatchedPaymentsCSV(c, q)
    if err != nil {
        c.JSONError(http.StatusInternalServerError, err.Error())
        return
    }
    c.Header("Content-Type", "text/csv")
    c.Header("Content-Disposition", "attachment; filename=unmatched_payments.csv")
    c.Data(http.StatusOK, "text/csv", b)
}

// ManualMark 标记为已对账
// @Summary 手动标记支付已对账
// @Tags 对账
// @Accept json
// @Produce json
// @Param body body map[string]string true "{uuid}"
// @Router /api/v1/admin/payments/reconcile/mark [post]
func (pc *PaymentReconcileController) ManualMark(c *app.Context) {
    params := struct{ Uuid string `json:"uuid" binding:"required"` }{}
    if err := c.ShouldBindJSON(&params); err != nil {
        c.JSONError(http.StatusBadRequest, err.Error())
        return
    }
    if err := pc.Reconciler.ManualMarkReconciled(c, params.Uuid); err != nil {
        c.JSONError(http.StatusInternalServerError, err.Error())
        return
    }
    c.JSONSuccess(map[string]string{"marked": params.Uuid})
}
