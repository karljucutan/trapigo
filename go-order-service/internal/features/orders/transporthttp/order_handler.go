package transporthttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"go-order-service/internal/features/orders/application/command"
	"go-order-service/internal/features/orders/application/query"
	"go-order-service/internal/features/orders/domain"
)

const goOrdersRoutePrefix = "/api/v1/go-orders"

type OrderHandler struct {
	createOrderHandler     *command.CreateOrderHandler
	addOrderItemHandler    *command.AddOrderItemHandler
	updateOrderStatus      *command.UpdateOrderStatusHandler
	updateOrderItemHandler *command.UpdateOrderItemHandler
	removeOrderItemHandler *command.RemoveOrderItemHandler
	getOrderByIDHandler    *query.GetOrderByIDHandler
	listOrdersHandler      *query.ListOrdersHandler
	deleteOrderHandler     *command.DeleteOrderHandler
}

func NewOrderHandler(
	createOrderHandler *command.CreateOrderHandler,
	addOrderItemHandler *command.AddOrderItemHandler,
	updateOrderStatusHandler *command.UpdateOrderStatusHandler,
	updateOrderItemHandler *command.UpdateOrderItemHandler,
	removeOrderItemHandler *command.RemoveOrderItemHandler,
	getOrderByIDHandler *query.GetOrderByIDHandler,
	listOrdersHandler *query.ListOrdersHandler,
	deleteOrderHandler *command.DeleteOrderHandler,
) *OrderHandler {
	return &OrderHandler{
		createOrderHandler:     createOrderHandler,
		addOrderItemHandler:    addOrderItemHandler,
		updateOrderStatus:      updateOrderStatusHandler,
		updateOrderItemHandler: updateOrderItemHandler,
		removeOrderItemHandler: removeOrderItemHandler,
		getOrderByIDHandler:    getOrderByIDHandler,
		listOrdersHandler:      listOrdersHandler,
		deleteOrderHandler:     deleteOrderHandler,
	}
}

func (h *OrderHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /orders", h.handleListOrders)
	mux.HandleFunc("POST /orders", h.createOrder)
	mux.HandleFunc("GET /orders/{id}", h.handleGetOrderByID)
	mux.HandleFunc("PATCH /orders/{id}/status", h.handleUpdateOrderStatus)
	mux.HandleFunc("POST /orders/{id}/items", h.handleAddOrderItem)
	mux.HandleFunc("PATCH /orders/{id}/items/{itemId}", h.handleUpdateOrderItem)
	mux.HandleFunc("DELETE /orders/{id}/items/{itemId}", h.handleRemoveOrderItem)
	mux.HandleFunc("DELETE /orders/{id}", h.handleDeleteOrder)

	mux.HandleFunc("GET "+goOrdersRoutePrefix, h.handleListOrders)
	mux.HandleFunc("POST "+goOrdersRoutePrefix, h.createOrder)
	mux.HandleFunc("GET "+goOrdersRoutePrefix+"/{id}", h.handleGetOrderByID)
	mux.HandleFunc("PATCH "+goOrdersRoutePrefix+"/{id}/status", h.handleUpdateOrderStatus)
	mux.HandleFunc("POST "+goOrdersRoutePrefix+"/{id}/items", h.handleAddOrderItem)
	mux.HandleFunc("PATCH "+goOrdersRoutePrefix+"/{id}/items/{itemId}", h.handleUpdateOrderItem)
	mux.HandleFunc("DELETE "+goOrdersRoutePrefix+"/{id}/items/{itemId}", h.handleRemoveOrderItem)
	mux.HandleFunc("DELETE "+goOrdersRoutePrefix+"/{id}", h.handleDeleteOrder)
}

func (h *OrderHandler) handleListOrders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	orders, err := h.listOrdersHandler.Handle(ctx)
	if err != nil {
		h.writeErrorResponse(w, err)
		return
	}

	writeJSON(w, http.StatusOK, orders)
}

func (h *OrderHandler) handleGetOrderByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.PathValue("id")
	if idStr == "" {
		http.Error(w, "missing order id", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid order id", http.StatusBadRequest)
		return
	}

	order, err := h.getOrderByIDHandler.Handle(ctx, query.GetOrderByIDQuery{ID: id})
	if err != nil {
		h.writeErrorResponse(w, err)
		return
	}

	writeJSON(w, http.StatusOK, order)
}

func (h *OrderHandler) handleDeleteOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.PathValue("id")
	if idStr == "" {
		http.Error(w, "missing order id", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid order id", http.StatusBadRequest)
		return
	}

	if err := h.deleteOrderHandler.Handle(ctx, command.DeleteOrderCommand{ID: id}); err != nil {
		h.writeErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *OrderHandler) handleUpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.PathValue("id")
	if idStr == "" {
		http.Error(w, "missing order id", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid order id", http.StatusBadRequest)
		return
	}

	var input struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	order, err := h.updateOrderStatus.Handle(ctx, command.UpdateOrderStatusCommand{ID: id, Status: input.Status})
	if err != nil {
		h.writeErrorResponse(w, err)
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *OrderHandler) handleAddOrderItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.PathValue("id")
	if idStr == "" {
		http.Error(w, "missing order id", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid order id", http.StatusBadRequest)
		return
	}

	var input struct {
		ProductID      int64 `json:"product_id"`
		Quantity       int   `json:"quantity"`
		UnitPriceCents int64 `json:"unit_price_cents"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	order, err := h.addOrderItemHandler.Handle(ctx, command.AddOrderItemCommand{
		OrderID:        id,
		ProductID:      input.ProductID,
		Quantity:       input.Quantity,
		UnitPriceCents: input.UnitPriceCents,
	})
	if err != nil {
		h.writeErrorResponse(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, order)
}

func (h *OrderHandler) handleUpdateOrderItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.PathValue("id")
	if idStr == "" {
		http.Error(w, "missing order id", http.StatusBadRequest)
		return
	}
	orderID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid order id", http.StatusBadRequest)
		return
	}

	itemIDStr := r.PathValue("itemId")
	if itemIDStr == "" {
		http.Error(w, "missing item id", http.StatusBadRequest)
		return
	}
	itemID, err := strconv.ParseInt(itemIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid item id", http.StatusBadRequest)
		return
	}

	var input struct {
		Quantity       *int   `json:"quantity"`
		UnitPriceCents *int64 `json:"unit_price_cents"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	order, err := h.updateOrderItemHandler.Handle(ctx, command.UpdateOrderItemCommand{
		OrderID:        orderID,
		ItemID:         itemID,
		Quantity:       input.Quantity,
		UnitPriceCents: input.UnitPriceCents,
	})
	if err != nil {
		h.writeErrorResponse(w, err)
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *OrderHandler) handleRemoveOrderItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.PathValue("id")
	if idStr == "" {
		http.Error(w, "missing order id", http.StatusBadRequest)
		return
	}
	orderID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid order id", http.StatusBadRequest)
		return
	}

	itemIDStr := r.PathValue("itemId")
	if itemIDStr == "" {
		http.Error(w, "missing item id", http.StatusBadRequest)
		return
	}
	itemID, err := strconv.ParseInt(itemIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid item id", http.StatusBadRequest)
		return
	}

	order, err := h.removeOrderItemHandler.Handle(ctx, command.RemoveOrderItemCommand{OrderID: orderID, ItemID: itemID})
	if err != nil {
		h.writeErrorResponse(w, err)
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *OrderHandler) createOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var input struct {
		CustomerID int64 `json:"customer_id"`
		Items      []struct {
			ProductID      int64 `json:"product_id"`
			Quantity       int   `json:"quantity"`
			UnitPriceCents int64 `json:"unit_price_cents"`
		} `json:"items"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	createItems := make([]command.CreateItemInput, 0, len(input.Items))
	for _, item := range input.Items {
		createItems = append(createItems, command.CreateItemInput{
			ProductID:      item.ProductID,
			Quantity:       item.Quantity,
			UnitPriceCents: item.UnitPriceCents,
		})
	}

	order, err := h.createOrderHandler.Handle(ctx, command.CreateOrderCommand{
		CustomerID: input.CustomerID,
		Items:      createItems,
	})
	if err != nil {
		h.writeErrorResponse(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, order)
}

func (h *OrderHandler) writeErrorResponse(w http.ResponseWriter, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, domain.ErrOrderNotFound), errors.Is(err, domain.ErrOrderItemNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, domain.ErrInvalidCustomerID),
		errors.Is(err, domain.ErrOrderEmpty),
		errors.Is(err, domain.ErrInvalidQuantity),
		errors.Is(err, domain.ErrInvalidPrice),
		errors.Is(err, domain.ErrInvalidProductID):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		h.writeInternalServerError(w)
	}
}

func (h *OrderHandler) writeInternalServerError(w http.ResponseWriter) {
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
