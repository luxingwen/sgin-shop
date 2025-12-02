package service

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"sgin/model"
)

// TestConcurrentOrderCreation 确保并发创建订单时库存不会超卖
func TestConcurrentOrderCreation(t *testing.T) {
	ctx := setupTestApp(t)

	// 创建一个产品SKU
	sku := uuid.New().String()
	initialStock := int64(5)

	pi := &model.ProductItem{
		Uuid:      sku,
		Name:      "concurrency-sku",
		Price:     10.0,
		Stock:     initialStock,
		CreatedAt: time.Now().Format(time.DateTime),
		UpdatedAt: time.Now().Format(time.DateTime),
	}
	if err := ctx.DB.Create(pi).Error; err != nil {
		t.Fatalf("failed create product item: %v", err)
	}

	attempts := 10
	var wg sync.WaitGroup
	wg.Add(attempts)

	successCh := make(chan struct{}, attempts)

	for i := 0; i < attempts; i++ {
		go func() {
			defer wg.Done()
			svc := NewOrderService()
			req := &model.ReqOrderCreate{
				UserId: "tester",
				Receiver: model.OrderReceiver{
					ReceiverName: "T",
				},
				Items: []model.ReqOrderItemCreate{{ProductItemID: sku, Quantity: 1}},
			}
			_, err := svc.CreateOrder(ctx, req)
			if err == nil {
				successCh <- struct{}{}
			}
		}()
	}

	wg.Wait()
	close(successCh)

	successCount := 0
	for range successCh {
		successCount++
	}

	// reload SKU
	var after model.ProductItem
	if err := ctx.DB.Where("uuid = ?", sku).First(&after).Error; err != nil {
		t.Fatalf("failed reload product item: %v", err)
	}

	if int64(successCount) != initialStock-after.Stock {
		t.Fatalf("unexpected success count vs stock change: success=%d initial=%d after=%d", successCount, initialStock, after.Stock)
	}

	if after.Stock < 0 {
		t.Fatalf("stock went negative: %d", after.Stock)
	}

	// small wait to let DB settle
	time.Sleep(50 * time.Millisecond)
}
