# fibAPI
## tldr;
API for serving values from the Fibonacci sequence.

## Setup
If you have go installed, you can clone the repo, build it, and run it.
```
git clone git@github.com:macintoshpie/fibapi.git &&
  cd fibapi &&
  go build . &&
  ./fibapi -port 8080
```

## Usage
Use a `GET` request to any of the following endpoints. A JSON response is provided:
```
"index": (uint32) index of API request 
"value": (string) value of fibonacci sequence at index
```
### Endpoints
#### `/current`
Get current index and value
#### `/next`
Increment index and get the value
#### `/previous`
Decrement index and get the value

## Comments
### Server implementation
The server is a basic `net/http` server, all in `main.go`. Atomic methods are used for manipulating the current index, and the json encoder for writing responses. The api supports recovery by occasionally writing the current index to a file, which it attempts to read on each startup. Debugging endpoints were added to get info about the cache as well as profiling info.
### Caching implementation
A few different caches were tried including fastcache, hashicorp's lru, and a simple slice. After doing some profiling, it was determined the slice was more performant.  
For the caching strategy, I decided that only caching the "result values" (those returned in API response) was a problematic approach. For example, if our cache holds `n` values and we've made `k` calls, `k >> n`, to the `/next` endpoint we'd have the `n` most recent values in cache. If we then make `n` calls to `/previous`, any following calls to `/previous` will result in a full cache miss and the value must be calculated. This could be very expensive if our current index is a very high number.  
I thought the best approach would be to cache values in pairs because you only need `fib(i)` and `fib(i-1)` to calculate any of the following values. As a result, the cache is more resistent to "holes" in the sequence that would otherwise require calculating a value from the beginning. All that's required is searching for the largest index cached that's less than your index and you can start from there.  
I also added `cachePadding` so that values could be cached at intervals rather than every single one. This reduces memory usage by some factor and also gives the cache a higher tolerance for `/previous` calls, avoiding having to recalculate values all the way from the beginning. The downside is that you'll rarely get direct cache hits, so a bit of computation is still required from some starting pair. I'd be interested in writing some full tests to determine the ideal cache size and `cachePadding`, but I ran out of time.
### Testing/Benchmarking
I wrote some basic tests in `fib_test.go` for the fibonacci functionality.  
For performance testing, I put the app in a docker container with limited resources (1 CPU, 512mb Memory). I then used [wrk](https://github.com/wg/wrk), and [http_load](https://acme.com/software/http_load/) for load testing. Looking at `docker stats` to view container resource usage, all tests kept the CPU at 100%, and remained under the 512mb memory limit.
#### Only `/next` requests
I started a server at index 0, then ran wrk for 90 seconds. My API was able to handle ~1,000 requests per second:
```
(base) Teds-MacBook-Pro:wrk tedsummer$ ./wrk -t12 -c12 -d90s http://127.0.0.1:8080/next
Running 2m test @ http://127.0.0.1:8080/next
  12 threads and 12 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency    19.94ms   22.63ms 228.14ms   80.45%
    Req/Sec    86.46     59.18   470.00     84.84%
  92993 requests in 1.50m, 0.86GB read
Requests/sec:   1032.45
Transfer/sec:      9.73MB
(base) Teds-MacBook-Pro:wrk tedsummer$ curl http://127.0.0.1:8080/current
{"index":93005,"value":"3515575...<edited out>"}
```
#### High index followed by only `/previous` requests
This is the worst case for my API since it results in the most cache misses, meaning we have to brute force calculate large values. For this test, I set the starting index to 100,000 (with an empty cache) and hit the `/previous` endpoint for 90 seconds. It averaged out about 700 requests per second.
```
(base) Teds-MacBook-Pro:wrk tedsummer$ ./wrk -t12 -c12 -d90s http://127.0.0.1:8080/previous
Running 2m test @ http://127.0.0.1:8080/previous
  12 threads and 12 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency    29.61ms   72.49ms   1.65s    99.01%
    Req/Sec    61.97     26.66   220.00     73.28%
  65904 requests in 1.50m, 0.87GB read
Requests/sec:    731.52
Transfer/sec:      9.89MB
(base) Teds-MacBook-Pro:wrk tedsummer$ curl http://127.0.0.1:8080/current
{"index":34086,"value":"159668982821...<edited out>"}
```
#### Only `/current`
Again I set the starting index to 100,000 with an empty cache, then hit `/current` with wrk for 90 seconds. The results were pretty poor, about 400 rps. This explained below in the Bottlenecks section.
```
(base) Teds-MacBook-Pro:wrk tedsummer$ ./wrk -t12 -c12 -d90s http://127.0.0.1:8080/current
Running 2m test @ http://127.0.0.1:8080/current
  12 threads and 12 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency    36.44ms   67.20ms   1.31s    99.04%
    Req/Sec    38.22     10.79   110.00     70.83%
  40876 requests in 1.50m, 821.17MB read
Requests/sec:    453.65
Transfer/sec:      9.11MB
```
#### Mix of all endpoints
For this test, a random sequence of 1000 requests to each endpoint was used with the http_load tool. Unfortunately http_load doesn't seem to work very well. As a result it reported about 500 requests per second. I really believe these poor results are due to the tool, and that it's probably capable of at least 1000 rps. I'd like to find a better tool for testing multiple URLs (siege seems like a good option).
```
49134 fetches, 12 max parallel, 1.74852e+06 bytes, in 90.0042 seconds
35.5867 mean bytes/connection
545.908 fetches/sec, 19427.1 bytes/sec
msecs/connect: 1.23999 mean, 4095.29 max, 0.06 min
msecs/first-response: 6.07299 mean, 2380.89 max, 0.926 min
44134 bad byte counts
HTTP response codes:
  code 200 -- 49110
```
### Bottlenecks
Using the debugging endpoints, it was easy to see what was slowing it down. See the top10 below and the `next.cpu.svg` file (9 sec sample from `/next` load test)
```
Showing nodes accounting for 2960ms, 89.97% of 3290ms total
Dropped 48 nodes (cum <= 16.45ms)
Showing top 10 nodes out of 56
      flat  flat%   sum%        cum   cum%
    1080ms 32.83% 32.83%     1080ms 32.83%  math/big.mulAddVWW
     650ms 19.76% 52.58%      650ms 19.76%  math/big.subVV
     310ms  9.42% 62.01%     2210ms 67.17%  math/big.nat.divLarge
     280ms  8.51% 70.52%      280ms  8.51%  math/big.divWVW
     200ms  6.08% 76.60%      200ms  6.08%  syscall.Syscall
     150ms  4.56% 81.16%     2690ms 81.76%  math/big.nat.convertWords
     150ms  4.56% 85.71%      150ms  4.56%  time.now
      50ms  1.52% 87.23%       50ms  1.52%  encoding/json.(*encodeState).string
      50ms  1.52% 88.75%       50ms  1.52%  internal/poll.runtime_pollSetDeadline
      40ms  1.22% 89.97%       40ms  1.22%  math/big.greaterThan
```
Converting the big.Ints into strings was the most costly operation.  
After running some tests, I found that writing `[]byte` could be more than 2x faster and implemented my own basic addition on the slices. But in the end my method of adding digits was too slow so I had to scrap it.  
I think the solution to this problem could be a smarter implementation of the `[]byte` storing and addition, but I'd have to take a deeper look at go's big.nat implementation to see how they accomplish fast addition.