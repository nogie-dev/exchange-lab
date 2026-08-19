// Package numeric contains the fixed-point values used by the matching
// engine. Decimal strings are converted at the API boundary; the matching
// core never needs to parse or compare floating-point values.
package numeric

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

const (
	DefaultPriceScale    int32 = 8
	DefaultQuantityScale int32 = 8
	DefaultQuoteScale    int32 = 8
	DefaultConfigVersion int64 = 1
)

type PriceTicks int64
type QuantityLots int64
type QuoteAtoms int64

// Precision is the immutable numeric contract for one market configuration.
// Unit fields are expressed in the corresponding scaled integer unit.
type Precision struct {
	PriceScale      int32
	QuantityScale   int32
	QuoteScale      int32
	TickSizeUnits   int64
	LotSizeUnits    int64
	MinPriceTicks   PriceTicks
	MaxPriceTicks   PriceTicks
	MinQuantityLots QuantityLots
	MaxQuantityLots QuantityLots
	ConfigVersion   int64
}

func DefaultPrecision() Precision {
	return Precision{
		PriceScale:      DefaultPriceScale,
		QuantityScale:   DefaultQuantityScale,
		QuoteScale:      DefaultQuoteScale,
		TickSizeUnits:   1,
		LotSizeUnits:    1,
		MinPriceTicks:   1,
		MaxPriceTicks:   PriceTicks(math.MaxInt64),
		MinQuantityLots: 1,
		MaxQuantityLots: QuantityLots(math.MaxInt64),
		ConfigVersion:   DefaultConfigVersion,
	}
}

// WithDefaults keeps in-memory callers and old journal rows compatible with
// the first fixed-point configuration while still allowing explicit zero
// values for scales.
func (p Precision) WithDefaults() Precision {
	d := DefaultPrecision()
	if p.PriceScale == 0 && p.QuantityScale == 0 && p.QuoteScale == 0 &&
		p.TickSizeUnits == 0 && p.LotSizeUnits == 0 && p.ConfigVersion == 0 {
		return d
	}
	if p.TickSizeUnits == 0 {
		p.TickSizeUnits = d.TickSizeUnits
	}
	if p.LotSizeUnits == 0 {
		p.LotSizeUnits = d.LotSizeUnits
	}
	if p.MinPriceTicks == 0 {
		p.MinPriceTicks = d.MinPriceTicks
	}
	if p.MaxPriceTicks == 0 {
		p.MaxPriceTicks = d.MaxPriceTicks
	}
	if p.MinQuantityLots == 0 {
		p.MinQuantityLots = d.MinQuantityLots
	}
	if p.MaxQuantityLots == 0 {
		p.MaxQuantityLots = d.MaxQuantityLots
	}
	if p.ConfigVersion == 0 {
		p.ConfigVersion = d.ConfigVersion
	}
	return p
}

func (p Precision) Validate() error {
	if p.PriceScale < 0 || p.PriceScale > 18 {
		return fmt.Errorf("price scale must be between 0 and 18")
	}
	if p.QuantityScale < 0 || p.QuantityScale > 18 {
		return fmt.Errorf("quantity scale must be between 0 and 18")
	}
	if p.QuoteScale < 0 || p.QuoteScale > 18 {
		return fmt.Errorf("quote scale must be between 0 and 18")
	}
	if p.TickSizeUnits <= 0 || p.LotSizeUnits <= 0 {
		return fmt.Errorf("tick and lot sizes must be positive")
	}
	if p.MinPriceTicks <= 0 || p.MaxPriceTicks < p.MinPriceTicks {
		return fmt.Errorf("price bounds are invalid")
	}
	if p.MinQuantityLots <= 0 || p.MaxQuantityLots < p.MinQuantityLots {
		return fmt.Errorf("quantity bounds are invalid")
	}
	if p.ConfigVersion <= 0 {
		return fmt.Errorf("config version must be positive")
	}
	return nil
}

func (p Precision) ParsePrice(raw string) (PriceTicks, error) {
	value, err := ParseScaled(raw, p.PriceScale)
	if err != nil {
		return 0, fmt.Errorf("parse price: %w", err)
	}
	price := PriceTicks(value)
	if err := p.validatePrice(price); err != nil {
		return 0, err
	}
	return price, nil
}

func (p Precision) ParseQuantity(raw string) (QuantityLots, error) {
	value, err := ParseScaled(raw, p.QuantityScale)
	if err != nil {
		return 0, fmt.Errorf("parse quantity: %w", err)
	}
	quantity := QuantityLots(value)
	if err := p.validateQuantity(quantity); err != nil {
		return 0, err
	}
	return quantity, nil
}

func (p Precision) ValidatePrice(price PriceTicks) error {
	return p.validatePrice(price)
}

func (p Precision) ValidateQuantity(quantity QuantityLots) error {
	return p.validateQuantity(quantity)
}

func (p Precision) validatePrice(price PriceTicks) error {
	if price < p.MinPriceTicks || price > p.MaxPriceTicks {
		return fmt.Errorf("price is outside market bounds")
	}
	if price%PriceTicks(p.TickSizeUnits) != 0 {
		return fmt.Errorf("price does not match tick size")
	}
	return nil
}

func (p Precision) validateQuantity(quantity QuantityLots) error {
	if quantity < p.MinQuantityLots || quantity > p.MaxQuantityLots {
		return fmt.Errorf("quantity is outside market bounds")
	}
	if quantity%QuantityLots(p.LotSizeUnits) != 0 {
		return fmt.Errorf("quantity does not match lot size")
	}
	return nil
}

func (p Precision) FormatPrice(price PriceTicks) string {
	return FormatScaled(int64(price), p.PriceScale)
}

func (p Precision) FormatQuantity(quantity QuantityLots) string {
	return FormatScaled(int64(quantity), p.QuantityScale)
}

// QuoteAmount converts an execution's price and quantity into quote atoms.
// Positive values are rounded down at the quote scale. The calculation uses
// big.Int only for the checked multiplication; the orderbook remains int64.
func (p Precision) QuoteAmount(price PriceTicks, quantity QuantityLots) (QuoteAtoms, error) {
	if price < 0 || quantity < 0 {
		return 0, errors.New("price and quantity cannot be negative")
	}
	numerator := new(big.Int).Mul(big.NewInt(int64(price)), big.NewInt(int64(quantity)))
	numerator.Mul(numerator, pow10(p.QuoteScale))
	denominator := new(big.Int).Mul(pow10(p.PriceScale), pow10(p.QuantityScale))
	quotient := new(big.Int).Quo(numerator, denominator)
	if !quotient.IsInt64() {
		return 0, errors.New("quote amount overflows int64")
	}
	return QuoteAtoms(quotient.Int64()), nil
}

func ParseScaled(raw string, scale int32) (int64, error) {
	if scale < 0 || scale > 18 {
		return 0, fmt.Errorf("scale must be between 0 and 18")
	}
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, errors.New("value is required")
	}
	if strings.ContainsAny(value, "eE") {
		return 0, errors.New("exponent notation is not supported")
	}

	sign := int64(1)
	if value[0] == '-' || value[0] == '+' {
		if value[0] == '-' {
			sign = -1
		}
		value = value[1:]
	}
	if value == "" {
		return 0, errors.New("digits are required")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 {
		return 0, errors.New("invalid decimal value")
	}
	integerPart := parts[0]
	fractionalPart := ""
	if len(parts) == 2 {
		fractionalPart = parts[1]
		if fractionalPart != "" && !allDigits(fractionalPart) {
			return 0, errors.New("invalid fractional digits")
		}
	}
	if integerPart == "" && fractionalPart == "" {
		return 0, errors.New("digits are required")
	}
	if integerPart == "" {
		integerPart = "0"
	}
	if !allDigits(integerPart) {
		return 0, errors.New("invalid integer digits")
	}
	if int32(len(fractionalPart)) > scale {
		return 0, errors.New("too many fractional digits")
	}
	fractionalPart += strings.Repeat("0", int(scale)-len(fractionalPart))
	combined := strings.TrimLeft(integerPart+fractionalPart, "0")
	if combined == "" {
		return 0, nil
	}
	if sign < 0 {
		combined = "-" + combined
	}
	parsed, err := strconv.ParseInt(combined, 10, 64)
	if err != nil {
		return 0, errors.New("scaled value overflows int64")
	}
	return parsed, nil
}

// DecimalSign validates decimal syntax without applying a scale. It is useful
// for request shape validation before the market configuration is known.
// It returns -1, 0, or 1 for negative, zero, or positive values.
func DecimalSign(raw string) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" || strings.ContainsAny(value, "eE") {
		return 0, errors.New("invalid decimal value")
	}
	sign := 1
	if value[0] == '-' || value[0] == '+' {
		if value[0] == '-' {
			sign = -1
		}
		value = value[1:]
	}
	if value == "" {
		return 0, errors.New("digits are required")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || (parts[0] == "" && (len(parts) == 1 || parts[1] == "")) {
		return 0, errors.New("invalid decimal value")
	}
	for _, part := range parts {
		if part != "" && !allDigits(part) {
			return 0, errors.New("invalid decimal digits")
		}
	}
	if strings.Trim(value, "0.") == "" {
		return 0, nil
	}
	return sign, nil
}

func IsPositiveDecimal(raw string) bool {
	sign, err := DecimalSign(raw)
	return err == nil && sign > 0
}

func IsZeroDecimal(raw string) bool {
	sign, err := DecimalSign(raw)
	return err == nil && sign == 0
}

func MustPrice(raw string) PriceTicks {
	value, err := ParseScaled(raw, DefaultPriceScale)
	if err != nil {
		panic(err)
	}
	return PriceTicks(value)
}

func MustQuantity(raw string) QuantityLots {
	value, err := ParseScaled(raw, DefaultQuantityScale)
	if err != nil {
		panic(err)
	}
	return QuantityLots(value)
}

func MustQuote(raw string) QuoteAtoms {
	value, err := ParseScaled(raw, DefaultQuoteScale)
	if err != nil {
		panic(err)
	}
	return QuoteAtoms(value)
}

func FormatScaled(value int64, scale int32) string {
	if scale <= 0 {
		return strconv.FormatInt(value, 10)
	}
	negative := value < 0
	if negative {
		value = -value
	}
	digits := strconv.FormatInt(value, 10)
	if len(digits) <= int(scale) {
		digits = strings.Repeat("0", int(scale)+1-len(digits)) + digits
	}
	point := len(digits) - int(scale)
	digits = digits[:point] + "." + strings.TrimRight(digits[point:], "0")
	if strings.HasSuffix(digits, ".") {
		digits = strings.TrimSuffix(digits, ".")
	}
	if negative && digits != "0" {
		return "-" + digits
	}
	return digits
}

func allDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func pow10(scale int32) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
}
