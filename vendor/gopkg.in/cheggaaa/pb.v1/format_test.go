package pb_test

import (
	"fmt"
	"gopkg.in/cheggaaa/pb.v1"
	"strconv"
	"testing"
	"time"
)

func Test_DefaultsToInteger(t *testing.T) {
	value := int64(1000)
	expected := strconv.Itoa(int(value))
	actual := pb.Format(value).String()

	if actual != expected {
		t.Error(fmt.Sprintf("Expected {%s} was {%s}", expected, actual))
	}
}

func Test_CanFormatAsInteger(t *testing.T) {
	value := int64(1000)
	expected := strconv.Itoa(int(value))
	actual := pb.Format(value).To(pb.U_NO).String()

	if actual != expected {
		t.Error(fmt.Sprintf("Expected {%s} was {%s}", expected, actual))
	}
}

func Test_CanFormatAsBytes(t *testing.T) {
	value := int64(1000)
	expected := "1000 B"
	actual := pb.Format(value).To(pb.U_BYTES).String()

	if actual != expected {
		t.Error(fmt.Sprintf("Expected {%s} was {%s}", expected, actual))
	}
}

func Test_CanFormatDuration(t *testing.T) {
	value := 10 * time.Minute
	expected := "10m0s"
	actual := pb.Format(int64(value)).To(pb.U_DURATION).String()
	if actual != expected {
		t.Error(fmt.Sprintf("Expected {%s} was {%s}", expected, actual))
	}
}

func Test_DefaultUnitsWidth(t *testing.T) {
	value := 10
	expected := "     10"
	actual := pb.Format(int64(value)).Width(7).String()
	if actual != expected {
		t.Error(fmt.Sprintf("Expected {%s} was {%s}", expected, actual))
	}
}
