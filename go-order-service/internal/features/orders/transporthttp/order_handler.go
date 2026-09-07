package transporthttp

import (
	"encoding/json"
	stderrors "errors"
	"net/http"
	"strconv"

	"go-order-service/internal/features/orders/application/command"
	"go-order-service/internal/features/orders/application/query"
	"go-order-service/internal/features/orders/domain"

	"github.com/karljucutan/buildingblocks/correlation"
	bberrors "github.com/karljucutan/buildingblocks/errors"
	"github.com/karljucutan/buildingblocks/problemdetails"
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
		h.writeErrorResponse(r, w, err)
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
		h.writeErrorResponse(r, w, err)
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
		h.writeErrorResponse(r, w, err)
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

	var cmd command.UpdateOrderStatusCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	cmd.ID = id

	order, err := h.updateOrderStatus.Handle(ctx, cmd)
	if err != nil {
		h.writeErrorResponse(r, w, err)
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

	var cmd command.AddOrderItemCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	cmd.OrderID = id

	order, err := h.addOrderItemHandler.Handle(ctx, cmd)
	if err != nil {
		h.writeErrorResponse(r, w, err)
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

	var cmd command.UpdateOrderItemCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	cmd.OrderID = orderID
	cmd.ItemID = itemID

	order, err := h.updateOrderItemHandler.Handle(ctx, cmd)
	if err != nil {
		h.writeErrorResponse(r, w, err)
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
		h.writeErrorResponse(r, w, err)
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *OrderHandler) createOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var input struct {
		CustomerID int64                     `json:"customer_id"`
		Items      []command.CreateItemInput `json:"items"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	order, err := h.createOrderHandler.Handle(ctx, command.CreateOrderCommand{
		CustomerID: input.CustomerID,
		Items:      input.Items,
	})
	if err != nil {
		h.writeErrorResponse(r, w, err)
		return
	}

	writeJSON(w, http.StatusCreated, order)
}

func (h *OrderHandler) writeErrorResponse(r *http.Request, w http.ResponseWriter, err error) {
	requestID := correlation.ExtractOrGenerateRequestID(r)
	ctx := bberrors.WithInternalError(r.Context(), err)
	*r = *r.WithContext(ctx)
	var pd *problemdetails.ProblemDetails

	switch {
	case err == nil:
		return
	case stderrors.Is(err, domain.ErrOrderNotFound), stderrors.Is(err, domain.ErrOrderItemNotFound):
		pd = problemdetails.New().
			WithStatus(http.StatusNotFound).
			WithType("about:blank").
			WithTitle("Order Not Found").
			WithDetail(err.Error()).
			WithInstance("urn:request:" + requestID).
			Build()
		writeJSON(w, http.StatusNotFound, pd)
	case stderrors.Is(err, domain.ErrInvalidCustomerID),
		stderrors.Is(err, domain.ErrOrderEmpty),
		stderrors.Is(err, domain.ErrInvalidQuantity),
		stderrors.Is(err, domain.ErrInvalidPrice),
		stderrors.Is(err, domain.ErrInvalidProductID):
		pd = problemdetails.New().
			WithStatus(http.StatusBadRequest).
			WithType("about:blank").
			WithTitle("Invalid Request").
			WithDetail(err.Error()).
			WithInstance("urn:request:" + requestID).
			Build()
		writeJSON(w, http.StatusBadRequest, pd)
	default:
		pd = problemdetails.New().
			WithStatus(http.StatusInternalServerError).
			WithType("about:blank").
			WithTitle("Internal Server Error").
			WithDetail("An unexpected error occurred").
			WithInstance("urn:request:" + requestID).
			Build()
		writeJSON(w, http.StatusInternalServerError, pd)
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
