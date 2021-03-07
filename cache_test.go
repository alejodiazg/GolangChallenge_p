package sample1

import (
	"testing"
	"time"
	"fmt"
	"sort"
)

// mockResult has the float64 and err to return
type mockResult struct {
	price float64
	err   error
}

type mockPriceService struct {
	numCalls    int
	mockResults map[string]mockResult // what price and err to return for a particular itemCode
	callDelay   time.Duration         // how long to sleep on each call so that we can simulate calls to be expensive
}

func (m *mockPriceService) GetPriceFor(itemCode string) (float64, error) {

	m.numCalls++            // increase the number of calls
	time.Sleep(m.callDelay) // sleep to simulate expensive call

	result, ok := m.mockResults[itemCode]
	if !ok {
		panic(fmt.Errorf("bug in the tests, we didn't have a mock result for [%v]", itemCode))
	}
	return result.price, result.err
}

func (m *mockPriceService) getNumCalls() int {
	return m.numCalls
}

func getPriceWithNoErr(t *testing.T, cache *TransparentCache, itemCode string) float64 {
	price, err := cache.GetPriceFor(itemCode)
	if err != nil {
		t.Error("error getting price for", itemCode)
	}
	return price
}

func getPricesWithNoErr(t *testing.T, cache *TransparentCache, itemCodes ...string) []float64 {
	prices, err := cache.GetPricesFor(itemCodes...)
	if err != nil {
		t.Error("error getting prices for", itemCodes)
	}
	return prices
}

func assertInt(t *testing.T, expected int, actual int, msg string) {
	if expected != actual {
		t.Error(msg, fmt.Sprintf("expected : %v, got : %v", expected, actual))
	}
}

func assertIntLessThan(t *testing.T, max int, count int, msg string) {
	if max <= count {
		t.Error(msg, fmt.Sprintf("expected : %v > %v, got : %v <=  %v", max, count, max, count))
	}
}

func assertFloat(t *testing.T, expected float64, actual float64, msg string) {
	if expected != actual {
		t.Error(msg, fmt.Sprintf("expected : %v, got : %v", expected, actual))
	}
}

func assertFloats(t *testing.T, expected []float64, actual []float64, msg string) {
	if len(expected) != len(actual) {
		t.Error(msg, fmt.Sprintf("expected : %v, got : %v", expected, actual))
		return
	}
	sort.Float64s(expected)
	sort.Float64s(actual)
	for i, expectedValue := range expected {
		if expectedValue != actual[i] {
			t.Error(msg, fmt.Sprintf("expected : %v, got : %v", expected, actual))
			return
		}
	}
}

// Check that we are caching results (we should not call the external service for all calls)
func TestGetPriceFor_CachesResults(t *testing.T) {
	mockService := &mockPriceService{
		mockResults: map[string]mockResult{
			"p1": {price: 5, err: nil},
		},
	}
	cache := NewTransparentCache(mockService, time.Minute)
	assertFloat(t, 5, getPriceWithNoErr(t, cache, "p1"), "wrong price returned")
	assertFloat(t, 5, getPriceWithNoErr(t, cache, "p1"), "wrong price returned")
	assertFloat(t, 5, getPriceWithNoErr(t, cache, "p1"), "wrong price returned")
	assertInt(t, 1, mockService.getNumCalls(), "wrong number of service calls")
}

// Check that cache returns an error if external service returns an error
func TestGetPriceFor_ReturnsErrorOnServiceError(t *testing.T) {
	mockService := &mockPriceService{
		mockResults: map[string]mockResult{
			"p1": {price: 0, err: fmt.Errorf("some error")},
		},
	}
	cache := NewTransparentCache(mockService, time.Minute)
	_, err := cache.GetPriceFor("p1")
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}

// Check that cache can return more than one price at once, caching appropriately
func TestGetPricesFor_GetsSeveralPricesAtOnceAndCachesThem(t *testing.T) {
	mockService := &mockPriceService{
		mockResults: map[string]mockResult{
			"p1": {price: 5, err: nil},
			"p2": {price: 7, err: nil},
		},
	}
	cache := NewTransparentCache(mockService, time.Minute)
	assertFloat(t, 5, getPriceWithNoErr(t, cache, "p1"), "wrong price returned")
	assertFloats(t, []float64{5, 7}, getPricesWithNoErr(t, cache, "p1", "p2"), "wrong price returned")
	assertFloats(t, []float64{5, 7}, getPricesWithNoErr(t, cache, "p1", "p2"), "wrong price returned")
	assertInt(t, 2, mockService.getNumCalls(), "wrong number of service calls")
}

// Check that we are expiring results when they exceed the max age
func TestGetPriceFor_DoesNotReturnOldResults(t *testing.T) {
	mockService := &mockPriceService{
		mockResults: map[string]mockResult{
			"p1": {price: 5, err: nil},
			"p2": {price: 7, err: nil},
		},
	}
	maxAge := time.Millisecond * 200
	maxAge70Pct := time.Millisecond * 140
	cache := NewTransparentCache(mockService, maxAge)
	// get price for "p1" twice (one external service call)
	assertFloat(t, 5, getPriceWithNoErr(t, cache, "p1"), "wrong price returned")
	assertFloat(t, 5, getPriceWithNoErr(t, cache, "p1"), "wrong price returned")
	assertInt(t, 1, mockService.getNumCalls(), "wrong number of service calls")
	// sleep 0.7 the maxAge
	time.Sleep(maxAge70Pct)
	// get price for "p1" and "p2", only "p2" should be retrieved from the external service (one more external call)
	assertFloat(t, 5, getPriceWithNoErr(t, cache, "p1"), "wrong price returned")
	assertFloat(t, 5, getPriceWithNoErr(t, cache, "p1"), "wrong price returned")
	assertFloat(t, 7, getPriceWithNoErr(t, cache, "p2"), "wrong price returned")
	assertFloat(t, 7, getPriceWithNoErr(t, cache, "p2"), "wrong price returned")
	assertInt(t, 2, mockService.getNumCalls(), "wrong number of service calls")
	// sleep 0.7 the maxAge
	time.Sleep(maxAge70Pct)
	// get price for "p1" and "p2", only "p1" should be retrieved from the cache ("p2" is still valid)
	assertFloat(t, 5, getPriceWithNoErr(t, cache, "p1"), "wrong price returned")
	assertFloat(t, 5, getPriceWithNoErr(t, cache, "p1"), "wrong price returned")
	assertFloat(t, 7, getPriceWithNoErr(t, cache, "p2"), "wrong price returned")
	assertInt(t, 3, mockService.getNumCalls(), "wrong number of service calls")
}

// Check that cache parallelize service calls when getting several values at once
func TestGetPricesFor_ParallelizeCalls(t *testing.T) {
	mockService := &mockPriceService{
		callDelay: time.Second, // each call to external service takes one full second
		mockResults: map[string]mockResult{
			"p1": {price: 5, err: nil},
			"p2": {price: 7, err: nil},
		},
	}
	cache := NewTransparentCache(mockService, time.Minute)
	start := time.Now()
	assertFloats(t, []float64{5, 7}, getPricesWithNoErr(t, cache, "p1", "p2"), "wrong price returned")
	elapsedTime := time.Since(start)
	if elapsedTime > (1200 * time.Millisecond) {
		t.Error("calls took too long, expected them to take a bit over one second")
	}
}

//TODO test errored calls

//TODO add tests for number of calls to service when one item fails
// Check that cache returns an error if external service returns an error
func TestGetPriceFor_StopsSubRoutinesOnError(t *testing.T) {
	mockService := &mockPriceService{
		mockResults: map[string]mockResult{
			"c1": {price: 1, err: nil},
			"c2": {price: 1, err: nil},
			"c3": {price: 1, err: nil},
			"e1": {price: 0, err: fmt.Errorf("some error")},
			"c4": {price: 1, err: nil},
			"c5": {price: 1, err: nil},
			"c6": {price: 1, err: nil},
			"c7": {price: 1, err: nil},
			"c8": {price: 1, err: nil},
			"c9": {price: 1, err: nil},
			"c10": {price: 1, err: nil},
			"c11": {price: 1, err: nil},
		},
	}
	cache := NewTransparentCache(mockService, time.Minute)
	_, err := cache.GetPricesFor("c1", "c2", "c3", "e1", "c4", "c5", "c6", "c7", "c8", "c9", "c10", "c11")
	if err == nil {
		t.Errorf("expected error, got nil")
	}
	assertIntLessThan(t, 12, mockService.getNumCalls(), "wrong number of service calls")
}
