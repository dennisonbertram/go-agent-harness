# Easy tier tasks — 5 tasks
# Format: task_name|prompt
# Each prompt instructs the agent to work in /tmp/training-TASKNAME/, init a Go module,
# write implementation in main package, include _test.go, and run go test ./... to verify.

fibonacci-memoized|Work in /tmp/training-fibonacci/. Initialize a Go module (go mod init training/fibonacci). Write a file fib.go in package main implementing: func Fibonacci(n int) int using memoization with a map cache. Write fib_test.go with table-driven tests covering n=0,1,2,5,10,20 and verify F(20)=6765. Run go test ./... and report results. Acceptance: all tests pass, no global mutable state visible outside function.

generic-stack|Work in /tmp/training-stack/. Initialize a Go module (go mod init training/stack). Write stack.go in package main implementing a generic Stack[T any] struct with methods: Push(v T), Pop() (T, bool), Peek() (T, bool), Len() int. Write stack_test.go with tests for int and string stacks, testing empty-stack edge cases for Pop and Peek. Run go test ./... and report results. Acceptance: all tests pass, generics compile with Go 1.21+.

csv-parser|Work in /tmp/training-csvparser/. Initialize a Go module (go mod init training/csvparser). Write csv.go in package main implementing: func ParseCSV(s string) []map[string]string — parses a CSV string where the first row is the header and returns a slice of maps (column name -> value). Write csv_test.go with at least 3 table tests including empty input, single row, and multi-row cases. Run go test ./... and report results. Acceptance: all tests pass, handles trailing newline gracefully.

json-http-handler|Work in /tmp/training-httphandler/. Initialize a Go module (go mod init training/httphandler). Write handler.go in package main implementing an http.HandlerFunc that returns JSON {"status":"ok","ts":<unix-timestamp>} with Content-Type application/json and HTTP 200. Write handler_test.go using httptest.NewRecorder to test the response body is valid JSON with status=ok and ts is a positive integer. Run go test ./... and report results. Acceptance: all tests pass, JSON is well-formed.

bugfix-offbyone|Work in /tmp/training-bugfix/. Initialize a Go module (go mod init training/bugfix). Create buggy.go in package main with this broken function that has an off-by-one error: func SumSlice(nums []int) int { total := 0; for i := 0; i <= len(nums); i++ { total += nums[i] }; return total }. Fix the bug (change <= to <). Write buggy_test.go with tests for empty slice, single element, and multi-element slices. Run go test ./... and confirm tests pass. Acceptance: no panic, all tests pass.
