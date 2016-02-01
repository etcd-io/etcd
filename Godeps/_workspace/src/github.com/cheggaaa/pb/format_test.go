package pb

import (
	"fmt"
	"strconv"
	"testing"
)

func Test_DefaultsToInteger(t *testing.T) {
	value := int64(1000)
	expected := strconv.Itoa(int(value))
	actual := Format(value, -1)

	if actual != expected {
		t.Error(fmt.Sprintf("Expected {%s} was {%s}", expected, actual))
	}
}

func Test_CanFormatAsInteger(t *testing.T) {
	value := int64(1000)
	expected := strconv.Itoa(int(value))
	actual := Format(value, U_NO)

	if actual != expected {
		t.Error(fmt.Sprintf("Expected {%s} was {%s}", expected, actual))
	}
}

func Test_CanFormatAsBytes(t *testing.T) {
	value := int64(1000)
	expected := "1000 B"
	actual := Format(value, U_BYTES)

	if actual != expected {
		t.Error(fmt.Sprintf("Expected {%s} was {%s}", expected, actual))
	}
}
