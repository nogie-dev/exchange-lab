package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/nogie-dev/clob-trading/internal/engine"
	"github.com/nogie-dev/clob-trading/internal/models"
)

const maxRequestBodySize = 1 << 20

type handler struct {
	router      *engine.Router
	tickerAdder TickerAdder
}

// TickerAdder registers a ticker and prepares its worker for command intake.
type TickerAdder interface {
	AddTicker(context.Context, string) error
}

type statusResponse struct {
	Status string `json:"status"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func NewHandler(router *engine.Router) http.Handler {
	return newHandler(router, nil)
}

// NewHandlerWithTickerAdder adds the internal ticker management endpoint.
func NewHandlerWithTickerAdder(router *engine.Router, adder TickerAdder) http.Handler {
	return newHandler(router, adder)
}

func newHandler(router *engine.Router, adder TickerAdder) http.Handler {
	h := &handler{router: router, tickerAdder: adder}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ready", h.ready)
	mux.HandleFunc("GET /queries/orderbook", h.orderBook)
	mux.HandleFunc("POST /commands/orders/create", h.createOrder)
	mux.HandleFunc("POST /commands/orders/amend", h.amendOrder)
	mux.HandleFunc("POST /commands/orders/cancel", h.cancelOrder)
	mux.HandleFunc("POST /commands/tickers/add", h.addTicker)
	return mux
}

func (h *handler) ready(w http.ResponseWriter, _ *http.Request) {
	if err := h.router.Ready(); err != nil {
		writeEngineError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, statusResponse{Status: "ready"})
}

func (h *handler) orderBook(w http.ResponseWriter, r *http.Request) {
	ticker := strings.TrimSpace(r.URL.Query().Get("ticker"))
	depth, err := strconv.Atoi(r.URL.Query().Get("depth"))
	if ticker == "" || err != nil || depth <= 0 {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	snapshot, err := h.router.OrderBookSnapshot(ticker, depth)
	if err != nil {
		writeEngineError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (h *handler) createOrder(w http.ResponseWriter, r *http.Request) {
	var request models.CreateOrderRequest
	if !decodeJSON(w, r, &request) || !validCreateOrder(request) {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	h.acceptCommand(w, engine.Event{
		Type:     engine.NewOrder,
		Ticker:   request.Ticker,
		NewOrder: &request,
	})
}

func (h *handler) amendOrder(w http.ResponseWriter, r *http.Request) {
	var request models.EditOrderRequest
	if !decodeJSON(w, r, &request) || !validEditOrder(request) {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	h.acceptCommand(w, engine.Event{
		Type:    engine.EditOrder,
		Ticker:  request.Ticker,
		EditReq: &request,
	})
}

func (h *handler) cancelOrder(w http.ResponseWriter, r *http.Request) {
	var request models.CancelOrderRequest
	if !decodeJSON(w, r, &request) || strings.TrimSpace(request.CommandID) == "" || strings.TrimSpace(request.Ticker) == "" || strings.TrimSpace(request.OrderID) == "" {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	h.acceptCommand(w, engine.Event{
		Type:      engine.CancelOrder,
		Ticker:    request.Ticker,
		CancelReq: &request,
	})
}

type addTickerRequest struct {
	Ticker string `json:"ticker"`
}

func (h *handler) addTicker(w http.ResponseWriter, r *http.Request) {
	var request addTickerRequest
	if !decodeJSON(w, r, &request) || strings.TrimSpace(request.Ticker) == "" {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if h.tickerAdder == nil {
		writeError(w, http.StatusServiceUnavailable, "ticker management unavailable")
		return
	}
	if err := h.tickerAdder.AddTicker(r.Context(), strings.TrimSpace(request.Ticker)); err != nil {
		writeEngineError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, statusResponse{Status: "added"})
}

func (h *handler) acceptCommand(w http.ResponseWriter, event engine.Event) {
	if err := h.router.OrderRouter(event); err != nil {
		writeEngineError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, statusResponse{Status: "accepted"})
}

func validCreateOrder(request models.CreateOrderRequest) bool {
	validPrice := false
	switch request.OrderType {
	case models.Limit:
		validPrice = request.Price > 0
	case models.Market:
		validPrice = request.Price == 0
	}

	return strings.TrimSpace(request.CommandID) != "" &&
		strings.TrimSpace(request.Ticker) != "" &&
		strings.TrimSpace(request.UserID) != "" &&
		(request.OrderType == models.Limit || request.OrderType == models.Market) &&
		(request.Position == models.Bid || request.Position == models.Ask) &&
		validPrice && request.Amount > 0
}

func validEditOrder(request models.EditOrderRequest) bool {
	return strings.TrimSpace(request.CommandID) != "" &&
		strings.TrimSpace(request.Ticker) != "" &&
		strings.TrimSpace(request.OrderID) != "" &&
		request.Price > 0 &&
		(request.Amount == nil || *request.Amount > 0)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBodySize))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return false
	}
	return decoder.Decode(&struct{}{}) == io.EOF
}

func writeEngineError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, engine.ErrUnknownTicker):
		writeError(w, http.StatusNotFound, "unknown ticker")
	case errors.Is(err, engine.ErrTickerExists):
		writeError(w, http.StatusConflict, "ticker already exists")
	case errors.Is(err, engine.ErrEmptyTicker):
		writeError(w, http.StatusBadRequest, "invalid request")
	case errors.Is(err, engine.ErrEngineHalted):
		writeError(w, http.StatusServiceUnavailable, "engine halted")
	case errors.Is(err, engine.ErrRouterClosed):
		writeError(w, http.StatusServiceUnavailable, "engine unavailable")
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
