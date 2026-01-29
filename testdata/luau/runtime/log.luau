-- Log: tests rt:log with different levels
rt:log("info", "hello", 123)
rt:log("error", "something went wrong")
rt:output("done", nil)
