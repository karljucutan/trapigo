package transporthttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-order-service/internal/features/orders/application/command"
	"go-order-service/internal/features/orders/application/query"
	"go-order-service/internal/features/orders/domain"
	"go-order-service/internal/platform/transactionrepositories"

	"github.com/karljucutan/buildingblocks/middleware"
)

type inMemoryOrderRepo struct {
	orders map[int64]domain.Order
	nextID int64
}

func newInMemoryOrderRepo() *inMemoryOrderRepo {
	return &inMemoryOrderRepo{orders: map[int64]domain.Order{}}
}

func (r *inMemoryOrderRepo) Create(_ context.Context, order domain.Order) (domain.Order, error) {
	if r.nextID == 0 {
		r.nextID = 1
	}
	order.ID = r.nextID
	r.nextID++
	r.orders[order.ID] = order
	return order, nil
}

func (r *inMemoryOrderRepo) GetByID(_ context.Context, id int64) (domain.Order, error) {
	order, ok := r.orders[id]
	if !ok {
		return domain.Order{}, errors.New("order not found")
	}
	return order, nil
}

func (r *inMemoryOrderRepo) List(_ context.Context) ([]domain.Order, error) {
	orders := make([]domain.Order, 0, len(r.orders))
	for _, order := range r.orders {
		orders = append(orders, order)
	}
	return orders, nil
}

func (r *inMemoryOrderRepo) Update(_ context.Context, order domain.Order) (domain.Order, error) {
	r.orders[order.ID] = order
	return order, nil
}

func (r *inMemoryOrderRepo) Delete(_ context.Context, id int64) error {
	delete(r.orders, id)
	return nil
}

type inMemoryUnitOfWork struct {
	repo *inMemoryOrderRepo
}

// RunInTx implements the generic UoW[*transactionrepositories.TxRepositories] interface for testing.
func (u *inMemoryUnitOfWork) RunInTx(ctx context.Context, fn func(*transactionrepositories.TxRepositories) error) error {
	stores := &transactionrepositories.TxRepositories{Orders: u.repo}
	return fn(stores)
}

func newTestHandler(t *testing.T) (*OrderHandler, *inMemoryOrderRepo) {
	t.Helper()

	repo := newInMemoryOrderRepo()
	seed, err := domain.NewOrder(99, []domain.OrderItem{{ID: 1, ProductID: 10, Quantity: 2, UnitPriceCents: 2500}})
	if err != nil {
		t.Fatalf("seed order creation failed: %v", err)
	}
	seed, err = repo.Create(context.Background(), seed)
	if err != nil {
		t.Fatalf("repo.Create failed: %v", err)
	}

	uow := &inMemoryUnitOfWork{repo: repo}
	handler := NewOrderHandler(
		command.NewCreateOrderHandler(uow),
		command.NewAddOrderItemHandler(uow),
		command.NewUpdateOrderStatusHandler(uow),
		command.NewUpdateOrderItemHandler(uow),
		command.NewRemoveOrderItemHandler(uow),
		query.NewGetOrderByIDHandler(repo),
		query.NewListOrdersHandler(repo),
		command.NewDeleteOrderHandler(uow),
	)

	return handler, repo
}

func TestHandler_ListOrders(t *testing.T) {
	handler, _ := newTestHandler(t)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/orders", nil)
	res := httptest.NewRecorder()

	mux.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d; body=%s", http.StatusOK, res.Code, res.Body.String())
	}

	var orders []domain.Order
	if err := json.Unmarshal(res.Body.Bytes(), &orders); err != nil {
		t.Fatalf("failed to decode orders JSON: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(orders))
	}
	if orders[0].CustomerID != 99 {
		t.Fatalf("expected customer id 99, got %d", orders[0].CustomerID)
	}
}

func TestHandler_InternalErrorIsLogged(t *testing.T) {
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(previous)

	handler := &OrderHandler{listOrdersHandler: query.NewListOrdersHandler(&failingOrderRepo{err: errors.New("relation \"customer_order\" does not exist")})}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/go-orders", func(w http.ResponseWriter, r *http.Request) {
		handler.writeErrorResponse(r, w, errors.New("relation \"customer_order\" does not exist"))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/go-orders", nil)
	res := httptest.NewRecorder()

	middleware.LoggingMiddleware(mux).ServeHTTP(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d; body=%s", http.StatusInternalServerError, res.Code, res.Body.String())
	}

	output := buf.String()
	if !strings.Contains(output, "status=500") || !strings.Contains(output, "error=\"relation \\\"customer_order\\\" does not exist\"") {
		t.Fatalf("expected 500 log with internal error message, got: %q", output)
	}
}

func TestHandler_DoesNotLeakDatabaseErrors(t *testing.T) {
	failingRepo := &failingOrderRepo{err: errors.New("relation \"customer_order\" does not exist")}
	handler := &OrderHandler{listOrdersHandler: query.NewListOrdersHandler(failingRepo)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/go-orders", handler.handleListOrders)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/go-orders", nil)
	res := httptest.NewRecorder()

	mux.ServeHTTP(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d; body=%s", http.StatusInternalServerError, res.Code, res.Body.String())
	}

	// Parse Problem Details response
	var problemDetails map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&problemDetails); err != nil {
		t.Fatalf("expected valid JSON response, got error: %v; body=%s", err, res.Body.String())
	}

	// Verify Problem Details structure
	if _, hasStatus := problemDetails["status"]; !hasStatus {
		t.Fatalf("expected 'status' field in Problem Details, got %v", problemDetails)
	}
	if _, hasTitle := problemDetails["title"]; !hasTitle {
		t.Fatalf("expected 'title' field in Problem Details, got %v", problemDetails)
	}

	// Verify database error is not leaked
	if strings.Contains(res.Body.String(), "customer_order") {
		t.Fatalf("expected raw database error to be hidden, got %q", res.Body.String())
	}

	// Verify generic internal error message is present
	title := problemDetails["title"].(string)
	if !strings.Contains(title, "Internal Server Error") {
		t.Fatalf("expected generic internal error message in title, got %q", title)
	}
}

type failingOrderRepo struct {
	err error
}

func (r *failingOrderRepo) Create(context.Context, domain.Order) (domain.Order, error) {
	return domain.Order{}, r.err
}

func (r *failingOrderRepo) GetByID(context.Context, int64) (domain.Order, error) {
	return domain.Order{}, r.err
}

func (r *failingOrderRepo) List(context.Context) ([]domain.Order, error) {
	return nil, r.err
}

func (r *failingOrderRepo) Update(context.Context, domain.Order) (domain.Order, error) {
	return domain.Order{}, r.err
}

func (r *failingOrderRepo) Delete(context.Context, int64) error {
	return r.err
}

func TestHandler_GetOrderByID(t *testing.T) {
	handler, _ := newTestHandler(t)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/orders/1", nil)
	res := httptest.NewRecorder()

	mux.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d; body=%s", http.StatusOK, res.Code, res.Body.String())
	}

	var order domain.Order
	if err := json.Unmarshal(res.Body.Bytes(), &order); err != nil {
		t.Fatalf("failed to decode order JSON: %v", err)
	}
	if order.ID != 1 {
		t.Fatalf("expected order id 1, got %d", order.ID)
	}
	if order.CustomerID != 99 {
		t.Fatalf("expected customer id 99, got %d", order.CustomerID)
	}
}

func TestHandler_CreateOrder(t *testing.T) {
	handler, _ := newTestHandler(t)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	payload := bytes.NewBufferString(`{
		"customer_id": 123,
		"items": [
			{"product_id": 22, "quantity": 3, "unit_price_cents": 1500}
		]
	}`)

	req := httptest.NewRequest(http.MethodPost, "/orders", payload)
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	mux.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d; body=%s", http.StatusCreated, res.Code, res.Body.String())
	}

	var created domain.Order
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode created order JSON: %v", err)
	}
	if created.CustomerID != 123 {
		t.Fatalf("expected customer id 123, got %d", created.CustomerID)
	}
	if created.TotalAmountCents != 4500 {
		t.Fatalf("expected total 4500, got %d", created.TotalAmountCents)
	}
}

func TestHandler_DeleteOrder(t *testing.T) {
	handler, repo := newTestHandler(t)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	if _, err := repo.GetByID(context.Background(), 1); err != nil {
		t.Fatalf("seeded order with id 1 should exist: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/orders/1", nil)
	res := httptest.NewRecorder()

	mux.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d; body=%s", http.StatusNoContent, res.Code, res.Body.String())
	}

	if _, err := repo.GetByID(context.Background(), 1); err == nil {
		t.Fatal("expected order 1 to be deleted")
	}
}

func TestHandler_UpdateOrderStatus(t *testing.T) {
	handler, _ := newTestHandler(t)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	payload := bytes.NewBufferString(`{"status":"paid"}`)
	req := httptest.NewRequest(http.MethodPatch, "/orders/1/status", payload)
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	mux.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d; body=%s", http.StatusOK, res.Code, res.Body.String())
	}

	var order domain.Order
	if err := json.Unmarshal(res.Body.Bytes(), &order); err != nil {
		t.Fatalf("failed to decode updated order JSON: %v", err)
	}
	if order.Status != "paid" {
		t.Fatalf("expected status paid, got %q", order.Status)
	}
}

func TestHandler_AddItemToOrder(t *testing.T) {
	handler, _ := newTestHandler(t)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	payload := bytes.NewBufferString(`{"product_id":20,"quantity":3,"unit_price_cents":500}`)
	req := httptest.NewRequest(http.MethodPost, "/orders/1/items", payload)
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	mux.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d; body=%s", http.StatusCreated, res.Code, res.Body.String())
	}

	var order domain.Order
	if err := json.Unmarshal(res.Body.Bytes(), &order); err != nil {
		t.Fatalf("failed to decode order JSON: %v", err)
	}
	if len(order.Items) != 2 {
		t.Fatalf("expected 2 items total, got %d", len(order.Items))
	}
	if order.TotalAmountCents != 6500 {
		t.Fatalf("expected total 6500, got %d", order.TotalAmountCents)
	}
}

func TestHandler_UpdateOrderItemSupportsPartialPatch(t *testing.T) {
	handler, _ := newTestHandler(t)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	payload := bytes.NewBufferString(`{"quantity":4}`)
	req := httptest.NewRequest(http.MethodPatch, "/orders/1/items/1", payload)
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	mux.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d; body=%s", http.StatusOK, res.Code, res.Body.String())
	}

	var order domain.Order
	if err := json.Unmarshal(res.Body.Bytes(), &order); err != nil {
		t.Fatalf("failed to decode order JSON: %v", err)
	}
	if len(order.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(order.Items))
	}
	if order.Items[0].Quantity != 4 {
		t.Fatalf("expected quantity 4, got %d", order.Items[0].Quantity)
	}
	if order.Items[0].UnitPriceCents != 2500 {
		t.Fatalf("expected unit price 2500 to remain unchanged, got %d", order.Items[0].UnitPriceCents)
	}
	if order.TotalAmountCents != 10000 {
		t.Fatalf("expected total 10000, got %d", order.TotalAmountCents)
	}
}

func TestHandler_UpdateOrderItemSupportsPriceOnlyPartialPatch(t *testing.T) {
	handler, _ := newTestHandler(t)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	payload := bytes.NewBufferString(`{"unit_price_cents":4000}`)
	req := httptest.NewRequest(http.MethodPatch, "/orders/1/items/1", payload)
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	mux.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d; body=%s", http.StatusOK, res.Code, res.Body.String())
	}

	var order domain.Order
	if err := json.Unmarshal(res.Body.Bytes(), &order); err != nil {
		t.Fatalf("failed to decode order JSON: %v", err)
	}
	if order.Items[0].Quantity != 2 {
		t.Fatalf("expected quantity 2 to stay unchanged, got %d", order.Items[0].Quantity)
	}
	if order.Items[0].UnitPriceCents != 4000 {
		t.Fatalf("expected unit price 4000, got %d", order.Items[0].UnitPriceCents)
	}
	if order.TotalAmountCents != 8000 {
		t.Fatalf("expected total 8000, got %d", order.TotalAmountCents)
	}
}

func TestHandler_UpdateOrderItem(t *testing.T) {
	handler, _ := newTestHandler(t)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	payload := bytes.NewBufferString(`{"quantity":5,"unit_price_cents":400}`)
	req := httptest.NewRequest(http.MethodPatch, "/orders/1/items/1", payload)
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	mux.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d; body=%s", http.StatusOK, res.Code, res.Body.String())
	}

	var order domain.Order
	if err := json.Unmarshal(res.Body.Bytes(), &order); err != nil {
		t.Fatalf("failed to decode order JSON: %v", err)
	}
	if order.Items[0].Quantity != 5 {
		t.Fatalf("expected quantity 5, got %d", order.Items[0].Quantity)
	}
	if order.TotalAmountCents != 2000 {
		t.Fatalf("expected total 2000, got %d", order.TotalAmountCents)
	}
}

func TestHandler_RemoveOrderItem(t *testing.T) {
	handler, _ := newTestHandler(t)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/orders/1/items/1", nil)
	res := httptest.NewRecorder()

	mux.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d; body=%s", http.StatusOK, res.Code, res.Body.String())
	}

	var order domain.Order
	if err := json.Unmarshal(res.Body.Bytes(), &order); err != nil {
		t.Fatalf("failed to decode order JSON: %v", err)
	}
	if len(order.Items) != 0 {
		t.Fatalf("expected order to have 0 items, got %d", len(order.Items))
	}
}
