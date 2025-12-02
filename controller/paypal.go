package controller

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sgin/pkg/app"
	"sgin/service"
	"strconv"
)

type PaypalController struct {
}

// Return 回调（PayPal webhook）
func (p *PaypalController) Return(ctx *app.Context) {
	body, err := ioutil.ReadAll(ctx.Request.Body)
	if err != nil {
		ctx.Logger.Error("Failed to read body:", err)
		ctx.JSONError(http.StatusInternalServerError, "Failed to read body")
		return
	}

	ctx.Logger.Info("Paypal Return:", string(body))

	// 尝试解析常见字段以获取订单号与交易号
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		ctx.Logger.Warn("Failed to parse paypal webhook json, will continue with raw body", err)
	}

	// 尝试定位 order id / invoice / custom 字段
	orderID := ""
	txnID := ""
	amount := 0.0

	// common PayPal structures: resource -> invoice_id / resource -> id / resource -> purchase_units
	if res, ok := payload["resource"].(map[string]interface{}); ok {
		if v, ok := res["invoice_id"].(string); ok && v != "" {
			orderID = v
		}
		if v, ok := res["id"].(string); ok && v != "" {
			txnID = v
		}
		// try purchase_units[0].reference_id or amount.value
		if pus, ok := res["purchase_units"].([]interface{}); ok && len(pus) > 0 {
			if pu, ok := pus[0].(map[string]interface{}); ok {
				if v, ok := pu["reference_id"].(string); ok && v != "" {
					orderID = orderIDOr(orderID, v)
				}
				if amountMap, ok := pu["amount"].(map[string]interface{}); ok {
					if val, ok := amountMap["value"].(string); ok {
						if f, err := parseFloatString(val); err == nil {
							amount = f
						}
					}
				}
			}
		}
	}

	// fallback common fields
	if orderID == "" {
		if v, ok := payload["invoice_id"].(string); ok {
			orderID = v
		}
		if v, ok := payload["custom"].(string); ok {
			orderID = orderIDOr(orderID, v)
		}
	}

	// 调用支付服务处理通知（若无法找到 orderID，会返回错误）
	psvc := service.NewPaymentService()
	if err := psvc.HandleNotification(ctx, "paypal", orderID, txnID, amount, "", string(body)); err != nil {
		ctx.Logger.Error("Failed to handle paypal notification", err)
		ctx.JSONError(http.StatusInternalServerError, "failed to process notification")
		return
	}

	ctx.JSONSuccess("ok")
}

// Cancel 回调
func (p *PaypalController) Cancel(ctx *app.Context) {
	body, err := ioutil.ReadAll(ctx.Request.Body)
	if err != nil {
		ctx.Logger.Error("Failed to read body:", err)
		ctx.JSONError(http.StatusInternalServerError, "Failed to read body")
		return
	}

	ctx.Logger.Info("Paypal Cancel:", string(body))
	ctx.JSONSuccess("ok")
}

func orderIDOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func parseFloatString(s string) (float64, error) {
	// delegated to strconv.ParseFloat to avoid extra imports in other files
	return strconv.ParseFloat(s, 64)
}
