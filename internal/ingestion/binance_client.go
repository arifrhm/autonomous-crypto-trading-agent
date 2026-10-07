package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/domain"
)

const (
	defaultBaseWSURL   = "wss://stream.binance.com:9443/stream"
	defaultMinBackoff  = 1 * time.Second
	defaultMaxBackoff  = 30 * time.Second
	defaultPingPeriod  = 20 * time.Second
	defaultWriteWait   = 5 * time.Second
	defaultPongWait    = 60 * time.Second
	defaultChannelSize = 1000
)

// binanceTradeEvent matches Binance WebSocket Trade stream payload.
type binanceTradeEvent struct {
	Stream string `json:"stream"`
	Data   struct {
		EventType string `json:"e"`
		EventTime int64  `json:"E"`
		Symbol    string `json:"s"`
		TradeID   int64  `json:"t"`
		Price     string `json:"p"`
		Quantity  string `json:"q"`
		BuyerID   int64  `json:"b"`
		SellerID  int64  `json:"a"`
		TradeTime int64  `json:"T"`
		IsBuyerMaker bool `json:"m"`
	} `json:"data"`
}

// Config defines options for the BinanceClient.
type Config struct {
	BaseWSURL   string
	Symbols     []string
	MinBackoff  time.Duration
	MaxBackoff  time.Duration
	PingPeriod  time.Duration
	ChannelSize int
}

// BinanceClient manages real-time WebSocket connection to Binance.
type BinanceClient struct {
	cfg     Config
	logger  *zap.Logger
	tradeCh chan domain.Trade

	mu      sync.Mutex
	conn    *websocket.Conn
	closing bool
}

// NewBinanceClient initializes a BinanceClient.
func NewBinanceClient(cfg Config, logger *zap.Logger) (*BinanceClient, error) {
	if len(cfg.Symbols) == 0 {
		return nil, fmt.Errorf("at least one symbol must be configured")
	}
	if cfg.BaseWSURL == "" {
		cfg.BaseWSURL = defaultBaseWSURL
	}
	if cfg.MinBackoff == 0 {
		cfg.MinBackoff = defaultMinBackoff
	}
	if cfg.MaxBackoff == 0 {
		cfg.MaxBackoff = defaultMaxBackoff
	}
	if cfg.PingPeriod == 0 {
		cfg.PingPeriod = defaultPingPeriod
	}
	if cfg.ChannelSize == 0 {
		cfg.ChannelSize = defaultChannelSize
	}

	return &BinanceClient{
		cfg:     cfg,
		logger:  logger.Named("binance_ws"),
		tradeCh: make(chan domain.Trade, cfg.ChannelSize),
	}, nil
}

// Trades returns read-only channel for consuming trades.
func (c *BinanceClient) Trades() <-chan domain.Trade {
	return c.tradeCh
}

// Start begins connecting and streaming. Reconnects automatically until ctx is cancelled.
func (c *BinanceClient) Start(ctx context.Context) error {
	defer close(c.tradeCh)

	backoff := c.cfg.MinBackoff

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Context cancelled, exiting connection loop")
			return ctx.Err()
		default:
		}

		err := c.connectAndRead(ctx)
		if err != nil && ctx.Err() == nil {
			c.logger.Warn("WebSocket disconnected, reconnecting with backoff",
				zap.Error(err),
				zap.Duration("backoff", backoff),
			)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.calculateJitter(backoff)):
			}

			// Exponential backoff doubling
			backoff *= 2
			if backoff > c.cfg.MaxBackoff {
				backoff = c.cfg.MaxBackoff
			}
		} else {
			// Successful reset if connection lasted reasonably
			backoff = c.cfg.MinBackoff
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

// buildStreamURL constructs combined stream endpoint URL.
func (c *BinanceClient) buildStreamURL() string {
	streams := make([]string, len(c.cfg.Symbols))
	for i, s := range c.cfg.Symbols {
		streams[i] = fmt.Sprintf("%s@trade", strings.ToLower(s))
	}

	if strings.Contains(c.cfg.BaseWSURL, "?streams=") {
		return c.cfg.BaseWSURL
	}
	return fmt.Sprintf("%s?streams=%s", c.cfg.BaseWSURL, strings.Join(streams, "/"))
}

// calculateJitter applies full jitter to avoid thundering herd.
func (c *BinanceClient) calculateJitter(duration time.Duration) time.Duration {
	if duration <= 0 {
		return 0
	}
	jitter := time.Duration(rand.Int63n(int64(duration / 2)))
	return duration/2 + jitter
}

// connectAndRead establishes connection and reads messages until an error occurs or ctx cancels.
func (c *BinanceClient) connectAndRead(ctx context.Context) error {
	url := c.buildStreamURL()
	c.logger.Info("Connecting to Binance WebSocket stream", zap.String("url", url))

	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.DialContext(ctx, url, http.Header{})
	if err != nil {
		if resp != nil {
			c.logger.Error("Dial failed with HTTP response", zap.Int("status_code", resp.StatusCode))
		}
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.closing = false
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		if c.conn != nil {
			_ = c.conn.Close()
			c.conn = nil
		}
		c.mu.Unlock()
	}()

	_ = conn.SetReadDeadline(time.Now().Add(defaultPongWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(defaultPongWait))
		return nil
	})

	// Keep-alive ping loop in goroutine
	done := make(chan struct{})
	defer close(done)

	go c.keepAlive(ctx, conn, done)

	for {
		select {
		case <-ctx.Done():
			c.safeClose(conn)
			return ctx.Err()
		default:
		}

		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read message error: %w", err)
		}

		trade, err := c.parseMessage(msgBytes)
		if err != nil {
			c.logger.Debug("Skipping unparseable message", zap.ByteString("raw", msgBytes), zap.Error(err))
			continue
		}

		select {
		case c.tradeCh <- *trade:
		case <-ctx.Done():
			return ctx.Err()
		default:
			c.logger.Warn("Trade channel buffer full, dropping oldest or slow consumer detected",
				zap.String("symbol", trade.Symbol),
				zap.Int64("trade_id", trade.TradeID),
			)
		}
	}
}

// keepAlive sends ping control messages periodically.
func (c *BinanceClient) keepAlive(ctx context.Context, conn *websocket.Conn, done <-chan struct{}) {
	ticker := time.NewTicker(c.cfg.PingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			c.mu.Lock()
			if c.conn == nil {
				c.mu.Unlock()
				return
			}
			err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(defaultWriteWait))
			c.mu.Unlock()

			if err != nil {
				c.logger.Warn("Failed to send ping keep-alive", zap.Error(err))
				return
			}
		}
	}
}

func (c *BinanceClient) safeClose(conn *websocket.Conn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closing && conn != nil {
		c.closing = true
		_ = conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "shutting down"),
			time.Now().Add(defaultWriteWait),
		)
	}
}

// parseMessage parses combined stream payload into domain.Trade.
func (c *BinanceClient) parseMessage(raw []byte) (*domain.Trade, error) {
	var event binanceTradeEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return nil, err
	}

	if event.Data.EventType != "trade" && event.Data.Symbol == "" {
		return nil, fmt.Errorf("payload is not a valid trade event")
	}

	price, err := strconv.ParseFloat(event.Data.Price, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid price %q: %w", event.Data.Price, err)
	}

	qty, err := strconv.ParseFloat(event.Data.Quantity, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid quantity %q: %w", event.Data.Quantity, err)
	}

	trade := &domain.Trade{
		EventTime:    time.UnixMilli(event.Data.EventTime),
		TradeTime:    time.UnixMilli(event.Data.TradeTime),
		Symbol:       strings.ToUpper(event.Data.Symbol),
		TradeID:      event.Data.TradeID,
		Price:        price,
		Quantity:     qty,
		BuyerOrderID: event.Data.BuyerID,
		SellerOrderID: event.Data.SellerID,
		IsBuyerMaker: event.Data.IsBuyerMaker,
	}

	return trade, nil
}
