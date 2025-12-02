package controller

import (
    "net/http"

    "sgin/model"
    "sgin/pkg/app"
    "sgin/service"
)

// ReturnController handles return/after-sales endpoints
type ReturnController struct {
    ReturnService *service.ReturnService
}

// ReqCreateReturn 请求结构
type ReqCreateReturn struct {
    OrderNo string `json:"order_no" binding:"required"`
    Reason  string `json:"reason" binding:"required"`
    Amount  float64 `json:"amount" binding:"required"`
    Items   []struct {
        ProductItemID string `json:"product_item_id" binding:"required"`
        Quantity      int    `json:"quantity" binding:"required"`
    } `json:"items" binding:"required"`
}

// CreateReturnRequest 用户发起退货申请
// @Summary 发起退货申请
// @Tags 退换货
// @Accept json
// @Produce json
// @Param body body controller.ReqCreateReturn true "退货申请"
// @Success 200 {object} model.Return
// @Router /api/v1/return/request [post]
func (rc *ReturnController) CreateReturnRequest(c *app.Context) {
    req := &ReqCreateReturn{}
    if err := c.ShouldBindJSON(req); err != nil {
        c.JSONError(http.StatusBadRequest, err.Error())
        return
    }

    items := make([]service.ReqReturnItem, 0, len(req.Items))
    for _, it := range req.Items {
        items = append(items, service.ReqReturnItem{ProductItemID: it.ProductItemID, Quantity: it.Quantity})
    }

    r, err := rc.ReturnService.CreateReturnRequest(c, req.OrderNo, c.GetString("user_id"), req.Reason, items, req.Amount)
    if err != nil {
        c.JSONError(http.StatusInternalServerError, err.Error())
        return
    }
    c.JSONSuccess(r)
}

// GetMyReturns 获取当前用户的退货列表
// @Summary 我的退货列表
// @Tags 退换货
// @Accept json
// @Produce json
// @Success 200 {array} model.Return
// @Router /api/v1/return/mylist [get]
func (rc *ReturnController) GetMyReturns(c *app.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        c.JSONError(http.StatusBadRequest, "user_id required")
        return
    }
    var returns []*model.Return
    if err := c.DB.Where("user_id = ?", userID).Find(&returns).Error; err != nil {
        c.JSONError(http.StatusInternalServerError, err.Error())
        return
    }
    c.JSONSuccess(returns)
}

// GetReturnInfo 获取退货详情
// @Summary 退货详情
// @Tags 退换货
// @Accept json
// @Produce json
// @Param params body model.ReqUserQueryParam false "params"
// @Success 200 {object} model.Return
// @Router /api/v1/return/info [post]
func (rc *ReturnController) GetReturnInfo(c *app.Context) {
    params := struct{ Uuid string `json:"uuid" binding:"required"` }{}
    if err := c.ShouldBindJSON(&params); err != nil {
        c.JSONError(http.StatusBadRequest, err.Error())
        return
    }
    var r model.Return
    if err := c.DB.Where("uuid = ?", params.Uuid).First(&r).Error; err != nil {
        c.JSONError(http.StatusInternalServerError, err.Error())
        return
    }
    c.JSONSuccess(r)
}

// AdminApproveReturn 管理员审批通过
// @Summary 管理员审批通过退货
// @Tags 退换货
// @Accept json
// @Produce json
// @Param body body map[string]interface{} true "{uuid,refund_amount}"
// @Success 200 {object} app.Response
// @Router /api/v1/admin/return/approve [post]
func (rc *ReturnController) AdminApproveReturn(c *app.Context) {
    params := struct{
        Uuid string `json:"uuid" binding:"required"`
        RefundAmount float64 `json:"refund_amount" binding:"required"`
    }{}
    if err := c.ShouldBindJSON(&params); err != nil {
        c.JSONError(http.StatusBadRequest, err.Error())
        return
    }
    if err := rc.ReturnService.ApproveReturn(c, params.Uuid, params.RefundAmount); err != nil {
        c.JSONError(http.StatusInternalServerError, err.Error())
        return
    }
    c.JSONSuccess("approved")
}

// AdminRejectReturn 管理员拒绝退货
// @Summary 管理员拒绝退货
// @Tags 退换货
// @Accept json
// @Produce json
// @Param body body map[string]interface{} true "{uuid,reason}"
// @Success 200 {object} app.Response
// @Router /api/v1/admin/return/reject [post]
func (rc *ReturnController) AdminRejectReturn(c *app.Context) {
    params := struct{
        Uuid string `json:"uuid" binding:"required"`
        Reason string `json:"reason" binding:"-"`
    }{}
    if err := c.ShouldBindJSON(&params); err != nil {
        c.JSONError(http.StatusBadRequest, err.Error())
        return
    }
    if err := rc.ReturnService.RejectReturn(c, params.Uuid, params.Reason); err != nil {
        c.JSONError(http.StatusInternalServerError, err.Error())
        return
    }
    c.JSONSuccess("rejected")
}
