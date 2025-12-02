package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"sgin/model"
	"sgin/pkg/app"
	"sgin/pkg/config"
	"sgin/pkg/db"
	"sgin/pkg/logger"
)

func setupTestApp(t *testing.T) *app.Context {
	t.Helper()
	// Try to locate config.yaml in current or parent directories
	if os.Getenv("CONFIG_FILE") == "" {
		cwd, _ := os.Getwd()
		dir := cwd
		found := ""
		for i := 0; i < 6; i++ { // search up to 6 levels
			candidate := filepath.Join(dir, "config.yaml")
			if _, err := os.Stat(candidate); err == nil {
				found = candidate
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
		if found != "" {
			_ = os.Setenv("CONFIG_FILE", found)
		}
	}

	if err := config.InitConfig(); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	cfg := config.GetConfig()

	if cfg.MySQL.Host == "" {
		t.Fatalf("no MySQL configured in config.yaml; please set MySQL host for integration tests")
	}

	dbConn, err := db.GetDB(cfg.MySQL)
	if err != nil || dbConn == nil {
		t.Fatalf("failed to connect to mysql from config: %v", err)
	}

	// migrate models (ensure test DB has needed tables)
	model.MigrateDbTable(dbConn)

	lg := logger.NewLogger(cfg.LogConfig)
	return &app.Context{
		DB:     dbConn,
		Logger: lg,
		Config: cfg,
		Ctx:    nil,
	}
}

func TestHandleNotification_CreateAndIdempotent(t *testing.T) {
	ctx := setupTestApp(t)

	// create order
	order := &model.Order{
		OrderNo:     "order123",
		UserID:      "user1",
		TotalAmount: 100.00,
		Status:      model.OrderStatusPending,
		CreatedAt:   time.Now().Format(time.DateTime),
		UpdatedAt:   time.Now().Format(time.DateTime),
	}
	if err := ctx.DB.Create(order).Error; err != nil {
		t.Fatalf("failed create order: %v", err)
	}

	svc := NewPaymentService()

	// first notification - should create payment and mark order paid
	if err := svc.HandleNotification(ctx, "alipay", "order123", "tx-1", 100.0, "SUCCESS", "{}"); err != nil {
		t.Fatalf("HandleNotification failed: %v", err)
	}

	// check payment exists
	var payments []model.Payment
	if err := ctx.DB.Where("order_id = ?", "order123").Find(&payments).Error; err != nil {
		t.Fatalf("failed query payments: %v", err)
	}
	if len(payments) != 1 {
		t.Fatalf("expected 1 payment, got %d", len(payments))
	}
	if payments[0].Status != model.PaymentStatusPaid {
		t.Fatalf("expected payment status paid, got %s", payments[0].Status)
	}

	// check order status
	var o model.Order
	if err := ctx.DB.Where("order_no = ?", "order123").First(&o).Error; err != nil {
		t.Fatalf("failed query order: %v", err)
	}
	if o.Status != model.OrderStatusPaid {
		t.Fatalf("expected order status paid, got %s", o.Status)
	}

	// second notification (duplicate) - should be idempotent
	if err := svc.HandleNotification(ctx, "alipay", "order123", "tx-1", 100.0, "SUCCESS", "{}"); err != nil {
		t.Fatalf("HandleNotification duplicate failed: %v", err)
	}

	// still one payment
	var payments2 []model.Payment
	if err := ctx.DB.Where("order_id = ?", "order123").Find(&payments2).Error; err != nil {
		t.Fatalf("failed query payments2: %v", err)
	}
	if len(payments2) != 1 {
		t.Fatalf("expected 1 payment after duplicate, got %d", len(payments2))
	}
}

func TestHandleNotification_AmountMismatchAndMissingOrder(t *testing.T) {
	ctx := setupTestApp(t)

	// create order with 100
	order := &model.Order{
		OrderNo:     "order-amt",
		UserID:      "user2",
		TotalAmount: 100.00,
		Status:      model.OrderStatusPending,
		CreatedAt:   time.Now().Format(time.DateTime),
		UpdatedAt:   time.Now().Format(time.DateTime),
	}
	if err := ctx.DB.Create(order).Error; err != nil {
		t.Fatalf("failed create order: %v", err)
	}

	svc := NewPaymentService()

	// amount mismatch
	if err := svc.HandleNotification(ctx, "wechat", "order-amt", "tx-amt", 90.0, "SUCCESS", "{}"); err == nil {
		t.Fatalf("expected amount mismatch error, got nil")
	}

	// missing order
	if err := svc.HandleNotification(ctx, "wechat", "no-such-order", "tx-2", 50.0, "SUCCESS", "{}"); err == nil {
		t.Fatalf("expected missing order error, got nil")
	}
}
