# Scenario 4 Metric Matrix

| Mode       | Delay   | PUT Latency     | GET Latency     | Convergence Time  |
|------------|---------|-----------------|-----------------|------------------|
| eventual   | 0ms     | 1.00ms          | 0.00ms          | 4.00ms            |
| eventual   | 500ms   | 0.00ms          | 1.00ms          | 513.00ms          |
| eventual   | 2000ms  | 0.00ms          | 1.00ms          | 2009.00ms         |
| strong     | 0ms     | 4.00ms          | 1.00ms          | 9.00ms            |
| strong     | 500ms   | 504.00ms        | 1.00ms          | 509.00ms          |
| strong     | 2000ms  | 2005.00ms       | 1.00ms          | 2009.00ms         |
