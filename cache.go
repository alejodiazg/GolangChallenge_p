package sample1

import (
	"fmt"
	"sync"
	"time"
)

// PriceService is a service that we can use to get prices for the items
// Calls to this service are expensive (they take time)
type PriceService interface {
	GetPriceFor(itemCode string) (float64, error)
}

// TransparentCache is a cache that wraps the actual service
// The cache will remember prices we ask for, so that we don't have to wait on every call
// Cache should only return a price if it is not older than "maxAge", so that we don't get stale prices
type TransparentCache struct {
	actualPriceService PriceService
	maxAge             time.Duration
	prices             map[string]priceCache
	mapMt			   sync.Mutex
}

//priceCache is a structure that stores a price and the time it was stored at to be used in TransparentCache prices map
type priceCache struct {
	price float64
	storedAt time.Time
}

//requestItem is a structure used to pass the tasks to the workers, indicating the index of the task for ordering
//and the item code to search for in the cache or real service
type requestItem struct {
	index int
	itemCode string
}

func NewTransparentCache(actualPriceService PriceService, maxAge time.Duration) *TransparentCache {
	return &TransparentCache{
		actualPriceService: actualPriceService,
		maxAge:             maxAge,
		prices:             map[string]priceCache{},
	}
}

// GetPriceFor gets the price for the item, either from the cache or the actual service if it was not cached or too old
func (c *TransparentCache) GetPriceFor(itemCode string) (float64, error) {
	c.mapMt.Lock()
	cachedPrice, ok := c.prices[itemCode]
	c.mapMt.Unlock()
	if ok && time.Now().Sub(cachedPrice.storedAt) < c.maxAge{
			return cachedPrice.price, nil
	}
	price, err := c.actualPriceService.GetPriceFor(itemCode)
	if err != nil {
		return 0, fmt.Errorf("getting price from service : %v", err.Error())
	}

	//check for concurrent map writes here as there could be 2 requests at the same time
	c.mapMt.Lock()
	c.prices[itemCode] = priceCache{
		price: price,
		storedAt: time.Now(),
	}
	c.mapMt.Unlock()
	return price, nil
}

// GetPricesFor gets the prices for several items at once, some might be found in the cache, others might not
// If any of the operations returns an error, it should return an error as well
func (c *TransparentCache) GetPricesFor(itemCodes ...string) ([]float64, error) {
	results := make([]float64, len(itemCodes))
	err := c.runGetPricesWorkers(results, itemCodes)
	return results, err
}

//runGetPricesWorkers sets up the workers to parallelize get prices for each requested item
// If any of the operations returns an error, it returns the error
func (c* TransparentCache) runGetPricesWorkers(results []float64, itemCodes []string) error{
	//create the channels
	errorChannel := make(chan error)
	waitDoneChannel := make(chan bool)
	jobsChannel := make(chan *requestItem)

	var err error
	//create workgroup
	var wg sync.WaitGroup

	workerCount := 5
	if len(results) < workerCount {
		workerCount = len(results)
	}

	for wc := 0 ; wc < workerCount ; wc++ {
		wg.Add(1)
		go c.pricesWorker(results, jobsChannel, errorChannel, &wg)
	}

	//wait for the wait group to finish and let the channel know
	go func() {
		wg.Wait()
		close(waitDoneChannel)
	}()

	for index, itemCode := range itemCodes {
		jobsChannel <- &requestItem{
			index:    index,
			itemCode: itemCode,
		}
	}

	close(jobsChannel)

	//wait for work group to finish or error in the error channel
	select {
		case <-waitDoneChannel:
			break
		case err = <-errorChannel:
			break
	}

	close(errorChannel)
	return err
}

//pricesWorker executes the logic to get the  the prices for an item and stores the result in the results array
//in the correct index position given by the index given by the itemsChannel
func (c *TransparentCache) pricesWorker(results []float64, itemsChannel chan *requestItem, errorChannel chan error, wg *sync.WaitGroup) {
	defer wg.Done()
	for item := range itemsChannel {
		price, err := c.GetPriceFor(item.itemCode)
		if err != nil {
			errorChannel <- err
		} else {
			results[item.index] =  price
		}
	}
}