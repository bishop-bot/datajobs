package ib

import (
	"testing"

	ibapi "github.com/bishop-bot/ibapi-go"
)

func TestHistoricalDataRequestToIBRequest(t *testing.T) {
	req := HistoricalDataRequest{
		Conid:      "265598",
		Exchange:   "SMART",
		Period:     "1d",
		Bar:        "5mins",
		StartTime:  "20240101-00:00:00",
		OutsideRth: true,
		Source:     "live",
	}

	ibReq := req.ToIBRequest()

	if ibReq.Conid != "265598" {
		t.Errorf("expected Conid 265598, got %s", ibReq.Conid)
	}
	if ibReq.Exchange != "SMART" {
		t.Errorf("expected Exchange SMART, got %s", ibReq.Exchange)
	}
	if ibReq.Period != "1d" {
		t.Errorf("expected Period 1d, got %s", ibReq.Period)
	}
	if ibReq.Bar != "5mins" {
		t.Errorf("expected Bar 5mins, got %s", ibReq.Bar)
	}
	if ibReq.StartTime != "20240101-00:00:00" {
		t.Errorf("expected StartTime 20240101-00:00:00, got %s", ibReq.StartTime)
	}
	if ibReq.OutsideRth != true {
		t.Error("expected OutsideRth true")
	}
	if ibReq.Source != "live" {
		t.Errorf("expected Source live, got %s", ibReq.Source)
	}
}

func TestHistoricalDataRequestToIBRequestDefaults(t *testing.T) {
	req := HistoricalDataRequest{
		Conid:    "265598",
		Exchange: "NASDAQ",
		Period:   "1w",
		Bar:      "1hour",
	}

	ibReq := req.ToIBRequest()

	if ibReq.OutsideRth != false {
		t.Error("expected default OutsideRth false")
	}
	if ibReq.StartTime != "" {
		t.Errorf("expected empty StartTime, got %s", ibReq.StartTime)
	}
	if ibReq.Source != "" {
		t.Errorf("expected empty Source, got %s", ibReq.Source)
	}
}

func TestHistoricalDataBarFields(t *testing.T) {
	// Test the HistoricalDataBar type has expected fields
	bar := ibapi.HistoricalDataBar{
		O: 100.5,
		C: 101.0,
		H: 101.5,
		L: 100.0,
		V: 1000000,
		T: 1717200000000,
	}

	if bar.O != 100.5 {
		t.Errorf("expected Open 100.5, got %f", bar.O)
	}
	if bar.C != 101.0 {
		t.Errorf("expected Close 101.0, got %f", bar.C)
	}
	if bar.H != 101.5 {
		t.Errorf("expected High 101.5, got %f", bar.H)
	}
	if bar.L != 100.0 {
		t.Errorf("expected Low 100.0, got %f", bar.L)
	}
	if bar.V != 1000000 {
		t.Errorf("expected Volume 1000000, got %f", bar.V)
	}
	if bar.T != 1717200000000 {
		t.Errorf("expected Timestamp 1717200000000, got %d", bar.T)
	}
}

func TestClientClosedError(t *testing.T) {
	err := &ClientClosedError{}

	if err.Error() != "IB client is closed" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}
