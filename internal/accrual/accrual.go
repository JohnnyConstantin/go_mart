package accrual

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/JohnnyConstantin/go_mart/internal/repository"
	"github.com/JohnnyConstantin/go_mart/models"
	"go.uber.org/zap"
)

type Processor struct {
	orderRepo    repository.OrderRepository
	client       *http.Client
	accrualURL   string
	sugar        *zap.SugaredLogger
	pollInterval time.Duration
	workerCount  int

	mu            sync.RWMutex
	retryAfter    time.Duration
	isRateLimited bool
	shutdownCh    chan struct{}
	workersWg     sync.WaitGroup
	jobsCh        chan repository.Order
}

func NewProcessor(orderRepo repository.OrderRepository, accrualURL string, sugar *zap.SugaredLogger, workerCount int) *Processor {
	if workerCount <= 0 {
		workerCount = 3 // Воркеры по умолчанию
	}

	return &Processor{
		orderRepo: orderRepo,
		client: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        workerCount,
				MaxIdleConnsPerHost: workerCount,
			},
		},
		accrualURL:   accrualURL,
		sugar:        sugar,
		pollInterval: 500 * time.Millisecond,
		workerCount:  workerCount,
		shutdownCh:   make(chan struct{}),
		jobsCh:       make(chan repository.Order, workerCount*2),
	}
}

// Start запускает воркер пул и dispatcher
func (p *Processor) Start(ctx context.Context) {
	p.sugar.Infof("Starting accrual processor with %d workers", p.workerCount)

	// Запускаем воркеры
	for i := 0; i < p.workerCount; i++ {
		p.workersWg.Add(1)
		go p.worker(ctx, i)
	}

	// Запускаем dispatcher
	p.workersWg.Add(1)
	go p.dispatcher(ctx)
}

// dispatcher получает заказы и распределяет по воркерам
func (p *Processor) dispatcher(ctx context.Context) {
	defer p.workersWg.Done()

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.sugar.Info("Dispatcher stopping due to context cancellation")
			close(p.jobsCh) // Закрываем канал jobs
			return
		case <-p.shutdownCh:
			p.sugar.Info("Dispatcher stopping due to shutdown signal")
			close(p.jobsCh) // Закрываем канал jobs
			return
		case <-ticker.C:
			if p.isRateLimitedNow() { // проверка флага ожидания
				p.sugar.Debug("Rate limited, skipping dispatch cycle")
				continue
			}

			p.fetchAndDispatchOrders(ctx)
		}
	}
}

// fetchAndDispatchOrders получает заказы и отправляет в канал jobs
func (p *Processor) fetchAndDispatchOrders(ctx context.Context) {
	orders, err := p.orderRepo.GetOrdersForProcessing(ctx)
	if err != nil {
		p.sugar.Errorf("Failed to get orders for processing: %v", err)
		return
	}

	for _, order := range orders {
		select {
		case <-ctx.Done():
			p.sugar.Info("Stopping order dispatch due to context cancellation")
			return
		case <-p.shutdownCh:
			p.sugar.Info("Stopping order dispatch due to shutdown signal")
			return
		case p.jobsCh <- order:
			// Заказ успешно отправлен воркеру (не логируем)
		default:
			// Канал jobs заполнен, пропускаем этот цикл
			p.sugar.Debug("Jobs channel full, skipping order dispatch")
			return
		}
	}
}

// worker обрабатывает заказы из канала jobs
func (p *Processor) worker(ctx context.Context, id int) {
	defer p.workersWg.Done()

	p.sugar.Infof("Worker %d started", id)
	defer p.sugar.Infof("Worker %d stopped", id)

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.shutdownCh:
			return
		case order, ok := <-p.jobsCh:
			if !ok {
				return // Канал jobs закрыт, завершаем работу
			}

			if p.isRateLimitedNow() {
				p.sugar.Debugf("Worker %d: rate limited, skipping order %s", id, order.Number)
				// Возвращаем заказ обратно в канал для последующей обработки
				go func() {
					time.Sleep(1 * time.Second) // Задержка перед отправкой обратно в канал,
					select {
					case p.jobsCh <- order:
					case <-ctx.Done():
					case <-p.shutdownCh:
					}
				}()
				continue
			}

			// Если все ок, то отправляем в сторонний сервис и процессим
			p.processOrder(ctx, order, id)
		}
	}
}

func (p *Processor) processOrder(ctx context.Context, order repository.Order, workerID int) {
	url := fmt.Sprintf("%s/api/orders/%s", p.accrualURL, order.Number)

	resp, err := p.client.Get(url)
	if err != nil {
		p.sugar.Errorf("Worker %d: failed to request order %s: %v", workerID, order.Number, err)
		return
	}
	defer resp.Body.Close()

	// Обработка ошибки 429
	if resp.StatusCode == http.StatusTooManyRequests {
		duration := p.parseRetryAfter(resp)
		p.setRateLimited(duration) // Устанавливаем флаг ожидания

		// Возвращаем заказ обратно в канал для последующей обработки
		go func() {
			time.Sleep(duration) // Ждем до окончания периода ограничения
			select {
			case p.jobsCh <- order:
			case <-ctx.Done():
			case <-p.shutdownCh:
			}
		}()
		return
	}

	// Обработка 204
	if resp.StatusCode == http.StatusNoContent {
		p.sugar.Debugf("Worker %d: order %s not registered", workerID, order.Number)
		return
	}

	// Обработка остальных неизвестных ошибок
	if resp.StatusCode != http.StatusOK {
		p.sugar.Errorf("Worker %d: failed to request order %s: %v", workerID, order.Number, resp.Status)
		return
	}

	result := models.AccrualResult{}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		p.sugar.Errorf("Worker %d: failed to decode response for order %s: %v", workerID, order.Number, err)
		return
	}

	// Обновляем заказ в БД
	order.Status = result.Status
	order.Accrual = result.Accrual

	if err := p.orderRepo.UpdateOrder(ctx, &order); err != nil {
		p.sugar.Errorf("Worker %d: failed to update order %s: %v", workerID, order.Number, err)
		return
	}

	p.sugar.Infof("Worker %d: order %s updated to status %s with accrual %f",
		workerID, order.Number, order.Status, order.Accrual)
}

// Проверка флага ожидания
func (p *Processor) isRateLimitedNow() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.isRateLimited
}

func (p *Processor) setRateLimited(duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Чтобы не получилось бесконечного выставления ожидания
	if p.isRateLimited {
		return
	}

	p.isRateLimited = true
	p.retryAfter = duration

	p.sugar.Infof("Rate limited detected, sleeping for %v", duration)

	// Запускаем таймер для сброса флага ожидания
	time.AfterFunc(duration, func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.isRateLimited = false
		p.sugar.Info("Rate limit period ended, resuming normal operation")
	})
}

func (p *Processor) parseRetryAfter(resp *http.Response) time.Duration {
	retryAfter := resp.Header.Get("Retry-After")

	if seconds, err := strconv.Atoi(retryAfter); err == nil {
		return time.Duration(seconds) * time.Second
	}

	return 60 * time.Second // По дефолту
}

// Stop Останавливает все воркеры и ждет их завершения
func (p *Processor) Stop() {
	p.sugar.Info("Shutting down accrual processor...")

	// Отправляем сигнал завершения
	close(p.shutdownCh)

	p.workersWg.Wait()
	p.sugar.Info("Accrual processor shutdown complete")
}

// GracefulShutdown обеспечивает корректное завершение
// (наверное пока лишняя прослойка, но в теории могут добавиться еще действия при Graceful shutdown)
func (p *Processor) GracefulShutdown() {
	p.Stop()
}
