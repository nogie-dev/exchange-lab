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
	"github.com/nogie-dev/clob-trading/internal/numeric"
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

// API requests use decimal strings. They are converted to canonical integer
// units before entering the engine or journal.
type createOrderRequest struct {
	CommandID string           `json:"command_id"`
	Ticker    string           `json:"ticker"`
	UserID    string           `json:"user_id"`
	OrderType models.OrderType `json:"order_type"`
	Position  models.Position  `json:"position"`
	Price     string           `json:"price"`
	Amount    string           `json:"amount"`
	Nonce     uint64           `json:"nonce"`
}

type editOrderRequest struct {
	CommandID string  `json:"command_id"`
	Ticker    string  `json:"ticker"`
	OrderID   string  `json:"order_id"`
	Price     string  `json:"price"`
	Amount    *string `json:"amount"`
}

type orderBookResponse struct {
	Ticker string                   `json:"ticker"`
	Bids   []orderBookLevelResponse `json:"bids"`
	Asks   []orderBookLevelResponse `json:"asks"`
}

type orderBookLevelResponse struct {
	Price            string `json:"price"`
	Amount           string `json:"amount"`
	CumulativeAmount string `json:"cumulativeAmount"`
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
	precision, err := h.router.MarketPrecision(ticker)
	if err != nil {
		writeEngineError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, formatOrderBook(snapshot, precision))
}

func (h *handler) createOrder(w http.ResponseWriter, r *http.Request) {
	var raw createOrderRequest
	if !decodeJSON(w, r, &raw) {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if !validRawCreateOrder(raw) {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	request, err := h.parseCreateOrder(raw)
	if err != nil {
		if errors.Is(err, engine.ErrUnknownTicker) || errors.Is(err, engine.ErrRouterClosed) {
			writeEngineError(w, err)
		} else {
			writeError(w, http.StatusBadRequest, "invalid request")
		}
		return
	}

	h.acceptCommand(w, engine.Event{
		Type:     engine.NewOrder,
		Ticker:   request.Ticker,
		NewOrder: &request,
	})
}

func (h *handler) amendOrder(w http.ResponseWriter, r *http.Request) {
	var raw editOrderRequest
	if !decodeJSON(w, r, &raw) {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if !validRawEditOrder(raw) {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	request, err := h.parseEditOrder(raw)
	if err != nil {
		if errors.Is(err, engine.ErrUnknownTicker) || errors.Is(err, engine.ErrRouterClosed) {
			writeEngineError(w, err)
		} else {
			writeError(w, http.StatusBadRequest, "invalid request")
		}
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

func (h *handler) parseCreateOrder(raw createOrderRequest) (models.CreateOrderRequest, error) {
	precision, err := h.router.MarketPrecision(strings.TrimSpace(raw.Ticker))
	if err != nil {
		return models.CreateOrderRequest{}, err
	}
	request := models.CreateOrderRequest{
		CommandID: raw.CommandID,
		Ticker:    strings.TrimSpace(raw.Ticker),
		UserID:    raw.UserID,
		OrderType: raw.OrderType,
		Position:  raw.Position,
		Nonce:     raw.Nonce,
	}
	if raw.Price != "" {
		value, err := numeric.ParseScaled(raw.Price, precision.PriceScale)
		if err != nil {
			return models.CreateOrderRequest{}, err
		}
		request.Price = numeric.PriceTicks(value)
	}
	if request.OrderType == models.Limit {
		if request.Price <= 0 {
			return models.CreateOrderRequest{}, errors.New("limit price is required")
		}
		if err := precision.ValidatePrice(request.Price); err != nil {
			return models.CreateOrderRequest{}, err
		}
	} else if request.OrderType == models.Market && request.Price != 0 {
		return models.CreateOrderRequest{}, errors.New("market order price must be zero")
	}
	request.Amount, err = precision.ParseQuantity(raw.Amount)
	if err != nil {
		return models.CreateOrderRequest{}, err
	}
	if !validCreateOrder(request) {
		return models.CreateOrderRequest{}, errors.New("invalid create order")
	}
	return request, nil
}

func validRawCreateOrder(raw createOrderRequest) bool {
	if strings.TrimSpace(raw.CommandID) == "" || strings.TrimSpace(raw.Ticker) == "" || strings.TrimSpace(raw.UserID) == "" || strings.TrimSpace(raw.Amount) == "" {
		return false
	}
	if (raw.OrderType != models.Limit && raw.OrderType != models.Market) || (raw.Position != models.Bid && raw.Position != models.Ask) {
		return false
	}
	if !numeric.IsPositiveDecimal(raw.Amount) {
		return false
	}
	if raw.Price == "" {
		return raw.OrderType == models.Market
	}
	sign, err := numeric.DecimalSign(raw.Price)
	if err != nil {
		return false
	}
	if raw.OrderType == models.Limit {
		return sign > 0
	}
	return sign == 0
}

func validRawEditOrder(raw editOrderRequest) bool {
	if strings.TrimSpace(raw.CommandID) == "" || strings.TrimSpace(raw.Ticker) == "" || strings.TrimSpace(raw.OrderID) == "" {
		return false
	}
	if !numeric.IsPositiveDecimal(raw.Price) {
		return false
	}
	if raw.Amount == nil {
		return true
	}
	return numeric.IsPositiveDecimal(*raw.Amount)
}

func (h *handler) parseEditOrder(raw editOrderRequest) (models.EditOrderRequest, error) {
	precision, err := h.router.MarketPrecision(strings.TrimSpace(raw.Ticker))
	if err != nil {
		return models.EditOrderRequest{}, err
	}
	priceValue, err := precision.ParsePrice(raw.Price)
	if err != nil {
		return models.EditOrderRequest{}, err
	}
	request := models.EditOrderRequest{
		CommandID: raw.CommandID,
		Ticker:    strings.TrimSpace(raw.Ticker),
		OrderID:   raw.OrderID,
		Price:     priceValue,
	}
	if raw.Amount != nil {
		amount, err := precision.ParseQuantity(*raw.Amount)
		if err != nil {
			return models.EditOrderRequest{}, err
		}
		request.Amount = &amount
	}
	if !validEditOrder(request) {
		return models.EditOrderRequest{}, errors.New("invalid edit order")
	}
	return request, nil
}

func formatOrderBook(snapshot engine.OrderBookSnapshot, precision numeric.Precision) orderBookResponse {
	response := orderBookResponse{Ticker: snapshot.Ticker}
	response.Bids = formatLevels(snapshot.Bids, precision)
	response.Asks = formatLevels(snapshot.Asks, precision)
	return response
}

func formatLevels(levels []engine.OrderBookLevel, precision numeric.Precision) []orderBookLevelResponse {
	formatted := make([]orderBookLevelResponse, len(levels))
	for i, level := range levels {
		formatted[i] = orderBookLevelResponse{
			Price:            precision.FormatPrice(level.Price),
			Amount:           precision.FormatQuantity(level.Amount),
			CumulativeAmount: precision.FormatQuantity(level.CumulativeAmount),
		}
	}
	return formatted
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
