package domain

import (
	"encoding/json"
	"testing"
)

// --- Constructor Tests ---

func TestNewDecimalFromInt(t *testing.T) {
	testCases := []struct {
		name     string
		value    int64
		expected string
	}{
		{"zero", 0, "0"},
		{"positive", 100, "100"},
		{"negative", -50, "-50"},
		{"large", 1000000, "1000000"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDecimalFromInt(tc.value)
			if d.String() != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, d.String())
			}
		})
	}
}

func TestNewDecimalFromString(t *testing.T) {
	testCases := []struct {
		name        string
		value       string
		expectError bool
		expected    string
	}{
		{"valid integer", "100", false, "100"},
		{"valid decimal", "123.45", false, "123.45"},
		{"negative", "-50.25", false, "-50.25"},
		{"zero", "0", false, "0"},
		{"invalid", "not-a-number", true, ""},
		{"empty", "", true, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := NewDecimalFromString(tc.value)

			if tc.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if d.String() != tc.expected {
					t.Errorf("expected %s, got %s", tc.expected, d.String())
				}
			}
		})
	}
}

// --- Arithmetic Tests ---

func TestDecimal_Add(t *testing.T) {
	d1 := NewDecimalFromInt(100)
	d2 := NewDecimalFromInt(50)

	result, err := d1.Add(d2)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	expected := NewDecimalFromInt(150)
	if !result.Equal(expected) {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestDecimal_Add_Negative(t *testing.T) {
	d1 := NewDecimalFromInt(100)
	d2 := NewDecimalFromInt(-50)

	result, err := d1.Add(d2)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	expected := NewDecimalFromInt(50)
	if !result.Equal(expected) {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestDecimal_Sub(t *testing.T) {
	d1 := NewDecimalFromInt(100)
	d2 := NewDecimalFromInt(30)

	result, err := d1.Sub(d2)
	if err != nil {
		t.Fatalf("Sub failed: %v", err)
	}

	expected := NewDecimalFromInt(70)
	if !result.Equal(expected) {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestDecimal_Sub_Negative(t *testing.T) {
	d1 := NewDecimalFromInt(50)
	d2 := NewDecimalFromInt(100)

	result, err := d1.Sub(d2)
	if err != nil {
		t.Fatalf("Sub failed: %v", err)
	}

	expected := NewDecimalFromInt(-50)
	if !result.Equal(expected) {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestDecimal_Mul(t *testing.T) {
	d1 := NewDecimalFromInt(10)
	d2 := NewDecimalFromInt(5)

	result, err := d1.Mul(d2)
	if err != nil {
		t.Fatalf("Mul failed: %v", err)
	}

	expected := NewDecimalFromInt(50)
	if !result.Equal(expected) {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestDecimal_Mul_WithDecimals(t *testing.T) {
	d1, _ := NewDecimalFromString("2.5")
	d2, _ := NewDecimalFromString("4")

	result, err := d1.Mul(d2)
	if err != nil {
		t.Fatalf("Mul failed: %v", err)
	}

	expected, _ := NewDecimalFromString("10")
	if !result.Equal(expected) {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestDecimal_Div(t *testing.T) {
	d1 := NewDecimalFromInt(100)
	d2 := NewDecimalFromInt(4)

	result, err := d1.Div(d2)
	if err != nil {
		t.Fatalf("Div failed: %v", err)
	}

	expected := NewDecimalFromInt(25)
	if !result.Equal(expected) {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestDecimal_Div_ByZero(t *testing.T) {
	d1 := NewDecimalFromInt(100)
	d2 := Zero

	_, err := d1.Div(d2)
	if err == nil {
		t.Fatal("expected error when dividing by zero")
	}
}

func TestDecimal_Div_WithDecimals(t *testing.T) {
	d1, _ := NewDecimalFromString("10")
	d2, _ := NewDecimalFromString("3")

	result, err := d1.Div(d2)
	if err != nil {
		t.Fatalf("Div failed: %v", err)
	}

	// Result should be approximately 3.333...
	resultStr := result.String()
	if len(resultStr) < 3 {
		t.Errorf("expected division result to have decimal places, got %s", resultStr)
	}
}

// --- Comparison Tests ---

func TestDecimal_IsZero(t *testing.T) {
	testCases := []struct {
		name     string
		value    Decimal
		expected bool
	}{
		{"zero", Zero, true},
		{"positive", NewDecimalFromInt(1), false},
		{"negative", NewDecimalFromInt(-1), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.value.IsZero()
			if result != tc.expected {
				t.Errorf("expected IsZero() = %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestDecimal_Equal(t *testing.T) {
	testCases := []struct {
		name     string
		d1       Decimal
		d2       Decimal
		expected bool
	}{
		{"equal integers", NewDecimalFromInt(100), NewDecimalFromInt(100), true},
		{"different integers", NewDecimalFromInt(100), NewDecimalFromInt(50), false},
		{"both zero", Zero, NewDecimalFromInt(0), true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.d1.Equal(tc.d2)
			if result != tc.expected {
				t.Errorf("expected Equal() = %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestDecimal_Cmp(t *testing.T) {
	testCases := []struct {
		name     string
		d1       Decimal
		d2       Decimal
		expected int
	}{
		{"less than", NewDecimalFromInt(50), NewDecimalFromInt(100), -1},
		{"equal", NewDecimalFromInt(100), NewDecimalFromInt(100), 0},
		{"greater than", NewDecimalFromInt(150), NewDecimalFromInt(100), 1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.d1.Cmp(tc.d2)
			if result != tc.expected {
				t.Errorf("expected Cmp() = %d, got %d", tc.expected, result)
			}
		})
	}
}

// --- JSON Marshaling Tests ---

func TestDecimal_MarshalJSON(t *testing.T) {
	testCases := []struct {
		name     string
		value    Decimal
		expected string
	}{
		{"integer", NewDecimalFromInt(100), "100"},
		{"decimal", mustDecimalFromString("123.45"), "123.45"},
		{"negative", NewDecimalFromInt(-50), "-50"},
		{"zero", Zero, "0"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := tc.value.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON failed: %v", err)
			}

			if string(data) != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, string(data))
			}
		})
	}
}

func TestDecimal_UnmarshalJSON(t *testing.T) {
	testCases := []struct {
		name        string
		json        string
		expected    string
		expectError bool
	}{
		{"integer", "100", "100", false},
		{"quoted integer", "\"100\"", "100", false},
		{"decimal", "123.45", "123.45", false},
		{"quoted decimal", "\"123.45\"", "123.45", false},
		{"negative", "-50", "-50", false},
		{"invalid", "\"not-a-number\"", "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var d Decimal
			err := d.UnmarshalJSON([]byte(tc.json))

			if tc.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("UnmarshalJSON failed: %v", err)
				}
				if d.String() != tc.expected {
					t.Errorf("expected %s, got %s", tc.expected, d.String())
				}
			}
		})
	}
}

func TestDecimal_JSON_RoundTrip(t *testing.T) {
	type TestStruct struct {
		Amount Decimal `json:"amount"`
	}

	original := TestStruct{
		Amount: mustDecimalFromString("123.45"),
	}

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Unmarshal
	var parsed TestStruct
	err = json.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Compare
	if !parsed.Amount.Equal(original.Amount) {
		t.Errorf("expected %s, got %s", original.Amount, parsed.Amount)
	}
}

// --- Database Scan/Value Tests ---

func TestDecimal_Value(t *testing.T) {
	d := NewDecimalFromInt(12345)

	value, err := d.Value()
	if err != nil {
		t.Fatalf("Value failed: %v", err)
	}

	strValue, ok := value.(string)
	if !ok {
		t.Fatalf("expected string value, got %T", value)
	}

	if strValue != "12345" {
		t.Errorf("expected '12345', got '%s'", strValue)
	}
}

func TestDecimal_Scan(t *testing.T) {
	testCases := []struct {
		name        string
		input       interface{}
		expected    string
		expectError bool
	}{
		{"nil", nil, "0", false},
		{"[]byte", []byte("123.45"), "123.45", false},
		{"string", "678.90", "678.90", false},
		{"int64", int64(100), "100", false},
		{"float64", float64(123.45), "123.45", false},
		{"unsupported type", true, "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var d Decimal
			err := d.Scan(tc.input)

			if tc.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("Scan failed: %v", err)
				}
				if d.String() != tc.expected {
					t.Errorf("expected %s, got %s", tc.expected, d.String())
				}
			}
		})
	}
}

// --- Round Tests ---

func TestDecimal_Round(t *testing.T) {
	testCases := []struct {
		name     string
		value    string
		places   int32
		expected string
	}{
		{"round to 2 places", "123.456", 2, "123.46"},
		{"round to 0 places", "123.456", 0, "123"},
		{"round to 1 place", "123.456", 1, "123.5"},
		{"already rounded", "100.50", 2, "100.50"},
		{"negative", "-123.456", 2, "-123.46"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			d := mustDecimalFromString(tc.value)

			rounded, err := d.Round(tc.places)
			if err != nil {
				t.Fatalf("Round failed: %v", err)
			}

			if rounded.String() != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, rounded.String())
			}
		})
	}
}

// --- Helper Functions ---

func mustDecimalFromString(s string) Decimal {
	d, err := NewDecimalFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}
