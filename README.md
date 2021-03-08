# Golang-Challenge

Solution to the challenge

##Decisions 

* The provided TransparentCache struct was modified

prices was changed from:
 
```prices             map[string]float64```

to

```prices             map[string]priceCache```

so the time it was stored at could be saved to be used on GetPrice to check the expiration time of the cache entry

added a ```mapMt			   sync.Mutex``` field to the struct, as the request were going to be pararel to block reads
and writes to the map as we should not access it by multiple go routines at the same time for reads/writes

* New struct added: requestItem, it is used with a channel to send jobs to the worker functions for parallelization

```
type requestItem struct {
	index int
	itemCode string
}
```

* A wait group was used to be able to wait for the goroutines to end inside ```runGetPricesWorkers```

## Notes

In the commits you can see a change was made at the end of coding, this was cause I was trying to correctly end the
executions of the worker go routines when one GetPrice returned an error, this was discarded as was generating to much
trouble for testing as the go routine behaviour is not managed by us, and could cause random fails on the tests. 
So I was not comfortable using this approach if testing was not completely correct.

With a bit more work I could have refined this but did not wanted to go above too much above of the time.