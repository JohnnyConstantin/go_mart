package accrual

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/JohnnyConstantin/go_mart/internal/repository"
	"github.com/JohnnyConstantin/go_mart/models"
	"go.uber.org/zap"
	"net/http"
	"time"
)

type Processor struct {
	orderRepo    repository.OrderRepository
	client       *http.Client
	accrualURL   string
	sugar        *zap.SugaredLogger
	pollInterval time.Duration
}

func NewProcessor(orderRepo repository.OrderRepository, accrualURL string, sugar *zap.SugaredLogger) *Processor {
	return &Processor{
		orderRepo:    orderRepo,
		client:       &http.Client{Timeout: 5 * time.Second},
		accrualURL:   accrualURL,
		sugar:        sugar,
		pollInterval: 500 * time.Millisecond, // Скорость опроса, поставил небольшую.
	}
}

// Start запускает фоновую обработку заказов
func (p *Processor) Start(ctx context.Context) {
	go p.processOrders(ctx)
}

func (p *Processor) processOrders(ctx context.Context) {
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.fetchOrders(ctx)
		}
	}
}

func (p *Processor) fetchOrders(ctx context.Context) {
	// Получаем заказы, требующие обработки
	orders, err := p.orderRepo.GetOrdersForProcessing(ctx)
	if err != nil {
		p.sugar.Errorf("Failed to get orders for processing: %v", err)
		return
	}

	for _, order := range orders {
		p.processOrder(ctx, order)
	}
}

func (p *Processor) processOrder(ctx context.Context, order repository.Order) {
	url := fmt.Sprintf("%s/api/orders/%s", p.accrualURL, order.Number)

	resp, err := p.client.Get(url)
	if err != nil {
		p.sugar.Errorf("Failed to request order %s: %v", order.Number, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		p.sugar.Debugf("Order %s not registered", order.Number)
		return
	}

	if resp.StatusCode != http.StatusOK {
		p.sugar.Errorf("Failed to request order %s: %v", order.Number, resp.Status)
		return
	}

	result := models.AccrualResult{}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		p.sugar.Errorf("Failed to decode response for order %s: %v", order.Number, err)
		return
	}

	// Обновляем заказ в БД
	order.Status = result.Status
	order.Accrual = result.Accrual

	if err := p.orderRepo.UpdateOrder(ctx, &order); err != nil {
		p.sugar.Errorf("Failed to update order %s: %v", order.Number, err)
		return
	}

	p.sugar.Infof("Order %s updated to status %s with accrual %f",
		order.Number, order.Status, order.Accrual)
}
