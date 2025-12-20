package domain

import (
	"database/sql/driver"
	"fmt"

	"github.com/cockroachdb/apd/v3"
)

// Decimal is a wrapper around apd.Decimal to provide easy database serialization
// and clean arithmetic methods for the domain layer.
type Decimal struct {
	apd.Decimal
}

// DefaultContext is used for arithmetic operations.
// We use HighPrecision to ensure accuracy.
// In a real banking app, you might want to configure this more strictly.
var DefaultContext = apd.BaseContext.WithPrecision(20)

// Zero constant for convenience
var Zero = NewDecimalFromInt(0)

// NewDecimalFromInt creates a Decimal from an int64
func NewDecimalFromInt(v int64) Decimal {
	d := Decimal{}
	d.SetInt64(v)
	return d
}

// NewDecimalFromString creates a Decimal from a string
func NewDecimalFromString(v string) (Decimal, error) {
	d := Decimal{}
	_, _, err := d.SetString(v)
	if err != nil {
		return d, fmt.Errorf("invalid decimal string %s: %w", v, err)
	}
	return d, nil
}

// String implements the fmt.Stringer interface.
func (d Decimal) String() string {
	return d.Decimal.String()
}

// Value implements the driver.Valuer interface for database serialization.
func (d Decimal) Value() (driver.Value, error) {
	return d.String(), nil
}

// Scan implements the sql.Scanner interface for database deserialization.
func (d *Decimal) Scan(value interface{}) error {
	if value == nil {
		d.SetInt64(0)
		return nil
	}

	switch v := value.(type) {
	case []byte:
		_, _, err := d.SetString(string(v))
		return err
	case string:
		_, _, err := d.SetString(v)
		return err
	case int64:
		d.SetInt64(v)
		return nil
	case float64:
		_, err := d.SetFloat64(v)
		if err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported type for Decimal scan: %T", value)
	}
}

// Arithmetic Helpers

func (d Decimal) Add(other Decimal) (Decimal, error) {
	res := Decimal{}
	if _, err := DefaultContext.Add(&res.Decimal, &d.Decimal, &other.Decimal); err != nil {
		return res, fmt.Errorf("add operation failed: %w", err)
	}
	return res, nil
}

func (d Decimal) Sub(other Decimal) (Decimal, error) {
	res := Decimal{}
	if _, err := DefaultContext.Sub(&res.Decimal, &d.Decimal, &other.Decimal); err != nil {
		return res, fmt.Errorf("sub operation failed: %w", err)
	}
	return res, nil
}

func (d Decimal) Mul(other Decimal) (Decimal, error) {
	res := Decimal{}
	if _, err := DefaultContext.Mul(&res.Decimal, &d.Decimal, &other.Decimal); err != nil {
		return res, fmt.Errorf("mul operation failed: %w", err)
	}
	return res, nil
}

func (d Decimal) Div(other Decimal) (Decimal, error) {
	if other.IsZero() {
		return Zero, fmt.Errorf("division by zero")
	}
	res := Decimal{}
	if _, err := DefaultContext.Quo(&res.Decimal, &d.Decimal, &other.Decimal); err != nil {
		return res, fmt.Errorf("div operation failed: %w", err)
	}
	return res, nil
}

func (d Decimal) IsZero() bool {
	return d.Decimal.IsZero()
}

func (d Decimal) Equal(other Decimal) bool {
	return d.Decimal.Cmp(&other.Decimal) == 0
}

func (d Decimal) Cmp(other Decimal) int {
	return d.Decimal.Cmp(&other.Decimal)
}

// MarshalJSON implements the json.Marshaler interface.
func (d Decimal) MarshalJSON() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (d *Decimal) UnmarshalJSON(data []byte) error {
	// Remove quotes if present
	s := string(data)
	if len(s) > 1 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	_, _, err := d.SetString(s)
	return err
}

// Round rounds the decimal to the specified number of places.
func (d Decimal) Round(places int32) (Decimal, error) {
	res := Decimal{}
	// Create a context for rounding
	ctx := apd.BaseContext.WithPrecision(20)
	ctx.Rounding = apd.RoundHalfUp

	// Create a quantization exponent 10^-places
	exp := -int64(places)
	quantizer := Decimal{}
	quantizer.SetFinite(0, int32(exp)) // 0 * 10^-places = 0.00...01 basically defines the scale?
	// apd.Quantize uses the exponent of the second argument.
	// We need a decimal that has the desired exponent.
	// Since SetFinite(coeff, exp) creates coeff * 10^exp.
	// To round to 2 decimals (10^-2), we pass something with exponent -2.

	// Better approach using Quantize:
	// "Quantize sets d to the value of x rounded to the precision of y."

	y := Decimal{}
	y.SetFinite(0, -places) // Value is 0, but Exponent is -places.

	if _, err := ctx.Quantize(&res.Decimal, &d.Decimal, y.Exponent); err != nil {
		return res, fmt.Errorf("quantize operation failed: %w", err)
	}
	return res, nil
}
