//! Tests for Luau bindings.

use super::*;

// =============================================================================
// State Lifecycle Tests
// =============================================================================

#[test]
fn test_new() {
    let state = State::new();
    assert!(state.is_ok());
}

#[test]
fn test_close() {
    let state = State::new().unwrap();
    drop(state);
    // No panic means success
}

#[test]
fn test_open_libs() {
    let state = State::new().unwrap();
    state.open_libs();
    // No panic means success
}

// =============================================================================
// Script Execution Tests
// =============================================================================

#[test]
fn test_do_string_simple() {
    let state = State::new().unwrap();
    state.open_libs();
    let result = state.do_string("x = 1 + 2");
    assert!(result.is_ok());

    state.get_global("x").unwrap();
    assert_eq!(state.to_number(-1), 3.0);
}

#[test]
fn test_do_string_arithmetic() {
    let state = State::new().unwrap();
    state.open_libs();

    let result = state.do_string(
        r#"
        a = 10
        b = 20
        c = a + b
        d = c * 2
        e = d / 4
    "#,
    );
    assert!(result.is_ok());

    state.get_global("e").unwrap();
    assert_eq!(state.to_number(-1), 15.0);
}

#[test]
fn test_do_string_function() {
    let state = State::new().unwrap();
    state.open_libs();

    let result = state.do_string(
        r#"
        function add(a, b)
            return a + b
        end
        result = add(3, 4)
    "#,
    );
    assert!(result.is_ok());

    state.get_global("result").unwrap();
    assert_eq!(state.to_number(-1), 7.0);
}

#[test]
fn test_do_string_table() {
    let state = State::new().unwrap();
    state.open_libs();

    let result = state.do_string(
        r#"
        t = {1, 2, 3}
        sum = 0
        for _, v in ipairs(t) do
            sum = sum + v
        end
    "#,
    );
    assert!(result.is_ok());

    state.get_global("sum").unwrap();
    assert_eq!(state.to_number(-1), 6.0);
}

#[test]
fn test_do_string_error() {
    let state = State::new().unwrap();
    let result = state.do_string("invalid syntax here ???");
    assert!(result.is_err());
}

// =============================================================================
// Compile and Bytecode Tests
// =============================================================================

#[test]
fn test_compile() {
    let state = State::new().unwrap();
    let bytecode = state.compile("x = 1", OptLevel::O2);
    assert!(bytecode.is_ok());
    assert!(!bytecode.unwrap().is_empty());
}

#[test]
fn test_load_bytecode() {
    let state = State::new().unwrap();
    state.open_libs();

    // Compile to bytecode
    let bytecode = state.compile("x = 42", OptLevel::O2).unwrap();

    // Load bytecode (pushes a function onto the stack)
    let result = state.load_bytecode(&bytecode, "chunk");
    assert!(result.is_ok());

    // Execute the loaded function
    let result = state.pcall(0, 0);
    assert!(result.is_ok());

    // Check the result
    state.get_global("x").unwrap();
    assert_eq!(state.to_number(-1), 42.0);
}

// =============================================================================
// Stack Operation Tests
// =============================================================================

#[test]
fn test_stack_push_pop() {
    let state = State::new().unwrap();

    assert_eq!(state.get_top(), 0);

    state.push_number(1.0);
    state.push_number(2.0);
    state.push_number(3.0);
    assert_eq!(state.get_top(), 3);

    state.pop(1);
    assert_eq!(state.get_top(), 2);

    state.set_top(0);
    assert_eq!(state.get_top(), 0);
}

#[test]
fn test_push_nil() {
    let state = State::new().unwrap();
    state.push_nil();
    assert!(state.is_nil(-1));
}

#[test]
fn test_push_boolean() {
    let state = State::new().unwrap();
    state.push_boolean(true);
    assert!(state.is_boolean(-1));
    assert!(state.to_boolean(-1));

    state.push_boolean(false);
    assert!(state.is_boolean(-1));
    assert!(!state.to_boolean(-1));
}

#[test]
fn test_push_number() {
    let state = State::new().unwrap();
    state.push_number(3.14159);
    assert!(state.is_number(-1));
    assert!((state.to_number(-1) - 3.14159).abs() < 0.0001);
}

#[test]
fn test_push_string() {
    let state = State::new().unwrap();
    state.push_string("hello world").unwrap();
    assert!(state.is_string(-1));
    assert_eq!(state.to_string(-1), Some("hello world".to_string()));
}

// =============================================================================
// Type Checking Tests
// =============================================================================

#[test]
fn test_type_checking() {
    let state = State::new().unwrap();

    state.push_nil();
    assert_eq!(state.get_type(-1), Type::Nil);
    state.pop(1);

    state.push_boolean(true);
    assert_eq!(state.get_type(-1), Type::Boolean);
    state.pop(1);

    state.push_number(42.0);
    assert_eq!(state.get_type(-1), Type::Number);
    state.pop(1);

    state.push_string("test").unwrap();
    assert_eq!(state.get_type(-1), Type::String);
    state.pop(1);

    state.new_table();
    assert_eq!(state.get_type(-1), Type::Table);
    state.pop(1);
}

#[test]
fn test_type_name() {
    let state = State::new().unwrap();

    state.push_nil();
    assert_eq!(state.type_name(-1), "nil");
    state.pop(1);

    state.push_boolean(true);
    assert_eq!(state.type_name(-1), "boolean");
    state.pop(1);

    state.push_number(42.0);
    assert_eq!(state.type_name(-1), "number");
    state.pop(1);

    state.push_string("test").unwrap();
    assert_eq!(state.type_name(-1), "string");
    state.pop(1);
}

// =============================================================================
// Table Operation Tests
// =============================================================================

#[test]
fn test_table_operations() {
    let state = State::new().unwrap();
    state.open_libs();

    // Create table
    state.new_table();

    // Set field
    state.push_number(42.0);
    state.set_field(-2, "answer").unwrap();

    // Get field
    state.get_field(-1, "answer").unwrap();
    assert_eq!(state.to_number(-1), 42.0);
}

#[test]
fn test_create_table() {
    let state = State::new().unwrap();

    // Create table with pre-allocated space
    state.create_table(10, 5);
    assert!(state.is_table(-1));
}

#[test]
fn test_table_iteration() {
    let state = State::new().unwrap();
    state.open_libs();

    let result = state.do_string("t = {a=1, b=2, c=3}");
    assert!(result.is_ok());

    state.get_global("t").unwrap();

    let mut count = 0;
    state.push_nil();
    while state.next(-2) {
        count += 1;
        state.pop(1); // Pop value, keep key
    }
    assert_eq!(count, 3);
}

// =============================================================================
// Global Variable Tests
// =============================================================================

#[test]
fn test_globals() {
    let state = State::new().unwrap();

    state.push_number(100.0);
    state.set_global("myGlobal").unwrap();

    state.get_global("myGlobal").unwrap();
    assert_eq!(state.to_number(-1), 100.0);
}

// =============================================================================
// Memory and GC Tests
// =============================================================================

#[test]
fn test_memory_usage() {
    let state = State::new().unwrap();
    let mem = state.memory_usage();
    assert!(mem > 0);
}

#[test]
fn test_gc() {
    let state = State::new().unwrap();
    state.open_libs();

    // Allocate some tables
    state.check_stack(50);
    for _ in 0..50 {
        state.new_table();
    }

    let before = state.memory_usage();
    state.set_top(0); // Clear stack
    state.gc();
    let after = state.memory_usage();

    // Memory should decrease after GC
    assert!(after <= before);
}

// =============================================================================
// Misc Tests
// =============================================================================

#[test]
fn test_version() {
    let version = State::version();
    assert!(!version.is_empty());
    assert!(version.contains("Luau"));
}

#[test]
fn test_obj_len() {
    let state = State::new().unwrap();
    state.open_libs();

    state.do_string("t = {1, 2, 3, 4, 5}").unwrap();
    state.get_global("t").unwrap();
    assert_eq!(state.obj_len(-1), 5);
}

// =============================================================================
// Complex Script Tests
// =============================================================================

#[test]
fn test_fibonacci() {
    let state = State::new().unwrap();
    state.open_libs();

    let result = state.do_string(
        r#"
        function fib(n)
            if n <= 1 then
                return n
            end
            return fib(n - 1) + fib(n - 2)
        end
        result = fib(10)
    "#,
    );
    assert!(result.is_ok());

    state.get_global("result").unwrap();
    assert_eq!(state.to_number(-1), 55.0);
}

#[test]
fn test_string_manipulation() {
    let state = State::new().unwrap();
    state.open_libs();

    let result = state.do_string(
        r#"
        s = "hello world"
        upper = string.upper(s)
        len = string.len(s)
        sub = string.sub(s, 1, 5)
    "#,
    );
    assert!(result.is_ok());

    state.get_global("upper").unwrap();
    assert_eq!(state.to_string(-1), Some("HELLO WORLD".to_string()));
    state.pop(1);

    state.get_global("len").unwrap();
    assert_eq!(state.to_number(-1), 11.0);
    state.pop(1);

    state.get_global("sub").unwrap();
    assert_eq!(state.to_string(-1), Some("hello".to_string()));
}

#[test]
fn test_closure() {
    let state = State::new().unwrap();
    state.open_libs();

    let result = state.do_string(
        r#"
        function counter()
            local count = 0
            return function()
                count = count + 1
                return count
            end
        end
        
        c = counter()
        a = c()
        b = c()
        c_val = c()
    "#,
    );
    assert!(result.is_ok());

    state.get_global("c_val").unwrap();
    assert_eq!(state.to_number(-1), 3.0);
}

#[test]
fn test_coroutine() {
    let state = State::new().unwrap();
    state.open_libs();

    let result = state.do_string(
        r#"
        co = coroutine.create(function()
            for i = 1, 3 do
                coroutine.yield(i)
            end
            return "done"
        end)
        
        _, a = coroutine.resume(co)
        _, b = coroutine.resume(co)
        _, c = coroutine.resume(co)
        _, d = coroutine.resume(co)
    "#,
    );
    assert!(result.is_ok());

    state.get_global("a").unwrap();
    assert_eq!(state.to_number(-1), 1.0);
    state.pop(1);

    state.get_global("d").unwrap();
    assert_eq!(state.to_string(-1), Some("done".to_string()));
}

// =============================================================================
// RegisterFunc Tests
// =============================================================================

#[test]
fn test_register_func_simple() {
    let mut state = State::new().unwrap();
    state.open_libs();

    state
        .register_func("get_answer", |s| {
            s.push_number(42.0);
            1
        })
        .unwrap();

    state.do_string("result = get_answer()").unwrap();

    state.get_global("result").unwrap();
    assert_eq!(state.to_number(-1), 42.0);
}

#[test]
fn test_register_func_with_args() {
    let mut state = State::new().unwrap();
    state.open_libs();

    state
        .register_func("add", |s| {
            let a = s.to_number(1);
            let b = s.to_number(2);
            s.push_number(a + b);
            1
        })
        .unwrap();

    state.do_string("result = add(10, 20)").unwrap();

    state.get_global("result").unwrap();
    assert_eq!(state.to_number(-1), 30.0);
}

#[test]
fn test_register_func_multi_return() {
    let mut state = State::new().unwrap();
    state.open_libs();

    state
        .register_func("minmax", |s| {
            let a = s.to_number(1);
            let b = s.to_number(2);
            if a < b {
                s.push_number(a);
                s.push_number(b);
            } else {
                s.push_number(b);
                s.push_number(a);
            }
            2
        })
        .unwrap();

    state.do_string("min, max = minmax(30, 10)").unwrap();

    state.get_global("min").unwrap();
    assert_eq!(state.to_number(-1), 10.0);
    state.pop(1);

    state.get_global("max").unwrap();
    assert_eq!(state.to_number(-1), 30.0);
}

#[test]
fn test_register_func_string_args() {
    let mut state = State::new().unwrap();
    state.open_libs();

    state
        .register_func("greet", |s| {
            let name = s.to_string(1).unwrap_or_default();
            let greeting = format!("Hello, {}!", name);
            s.push_string(&greeting).unwrap();
            1
        })
        .unwrap();

    state.do_string(r#"result = greet("World")"#).unwrap();

    state.get_global("result").unwrap();
    assert_eq!(state.to_string(-1), Some("Hello, World!".to_string()));
}

#[test]
fn test_register_func_overwrite() {
    let mut state = State::new().unwrap();
    state.open_libs();

    state
        .register_func("get_value", |s| {
            s.push_number(1.0);
            1
        })
        .unwrap();

    state
        .register_func("get_value", |s| {
            s.push_number(2.0);
            1
        })
        .unwrap();

    state.do_string("result = get_value()").unwrap();

    state.get_global("result").unwrap();
    assert_eq!(state.to_number(-1), 2.0);
}

#[test]
fn test_register_func_multiple() {
    let mut state = State::new().unwrap();
    state.open_libs();

    state
        .register_func("inc", |s| {
            s.push_number(s.to_number(1) + 1.0);
            1
        })
        .unwrap();

    state
        .register_func("dec", |s| {
            s.push_number(s.to_number(1) - 1.0);
            1
        })
        .unwrap();

    state
        .register_func("double", |s| {
            s.push_number(s.to_number(1) * 2.0);
            1
        })
        .unwrap();

    // dec(10) = 9, inc(9) = 10, double(10) = 20
    state.do_string("result = double(inc(dec(10)))").unwrap();

    state.get_global("result").unwrap();
    assert_eq!(state.to_number(-1), 20.0);
}

#[test]
fn test_register_func_cleanup_on_drop() {
    // Create and drop a state with registered functions
    {
        let mut state = State::new().unwrap();
        for i in 0..100 {
            state
                .register_func(&format!("func{}", i), |_| 0)
                .unwrap();
        }
    }

    // Create another state - should work fine
    let mut state2 = State::new().unwrap();
    state2.register_func("test", |_| 0).unwrap();
}

#[test]
fn test_unregister_func() {
    let mut state = State::new().unwrap();
    state.open_libs();

    state
        .register_func("my_func", |s| {
            s.push_number(42.0);
            1
        })
        .unwrap();

    // Verify it works
    state.do_string("result = my_func()").unwrap();
    state.get_global("result").unwrap();
    assert_eq!(state.to_number(-1), 42.0);
    state.pop(1);

    // Unregister
    state.unregister_func("my_func").unwrap();

    // Verify it's nil
    state.get_global("my_func").unwrap();
    assert!(state.is_nil(-1));
}

// =============================================================================
// Table Argument Tests
// =============================================================================

#[test]
fn test_register_func_table_arg() {
    let mut state = State::new().unwrap();
    state.open_libs();

    state
        .register_func("sum_table", |s| {
            if !s.is_table(1) {
                s.push_number(0.0);
                return 1;
            }

            let mut sum = 0.0;
            s.push_nil();
            while s.next(1) {
                if s.is_number(-1) {
                    sum += s.to_number(-1);
                }
                s.pop(1);
            }
            s.push_number(sum);
            1
        })
        .unwrap();

    state.do_string("result = sum_table({10, 20, 30, 40})").unwrap();

    state.get_global("result").unwrap();
    assert_eq!(state.to_number(-1), 100.0);
}

// =============================================================================
// Memory Management Tests
// =============================================================================

#[test]
fn test_memory_register_many() {
    let mut state = State::new().unwrap();
    state.open_libs();

    // Register 1000 functions
    for i in 0..1000 {
        let name = format!("func_{}", i);
        let val = i as f64;
        state
            .register_func(&name, move |s| {
                s.push_number(val);
                1
            })
            .unwrap();
    }

    // Verify some functions work
    state.do_string("result = func_999()").unwrap();

    state.get_global("result").unwrap();
    assert_eq!(state.to_number(-1), 999.0);
}

#[test]
fn test_memory_call_loop() {
    use std::sync::atomic::{AtomicU64, Ordering};
    use std::sync::Arc;

    let mut state = State::new().unwrap();
    state.open_libs();

    let counter = Arc::new(AtomicU64::new(0));
    let counter_clone = counter.clone();

    state
        .register_func("increment", move |s| {
            let val = counter_clone.fetch_add(1, Ordering::SeqCst) + 1;
            s.push_number(val as f64);
            1
        })
        .unwrap();

    // Call 10000 times
    state
        .do_string(
            r#"
        for i = 1, 10000 do
            increment()
        end
    "#,
        )
        .unwrap();

    assert_eq!(counter.load(Ordering::SeqCst), 10000);
}

#[test]
fn test_memory_string_args() {
    use std::sync::atomic::{AtomicUsize, Ordering};
    use std::sync::Arc;

    let mut state = State::new().unwrap();
    state.open_libs();

    let total_len = Arc::new(AtomicUsize::new(0));
    let total_len_clone = total_len.clone();

    state
        .register_func("process_string", move |s| {
            if let Some(str) = s.to_string(1) {
                total_len_clone.fetch_add(str.len(), Ordering::SeqCst);
                s.push_number(str.len() as f64);
            } else {
                s.push_number(0.0);
            }
            1
        })
        .unwrap();

    // Pass large strings many times
    state
        .do_string(
            r#"
        local bigStr = string.rep("x", 10000)
        for i = 1, 100 do
            process_string(bigStr)
        end
    "#,
        )
        .unwrap();

    assert_eq!(total_len.load(Ordering::SeqCst), 10000 * 100);
}

#[test]
fn test_memory_state_close() {
    // Create many states, register functions, drop them
    for _ in 0..100 {
        let mut state = State::new().unwrap();
        for j in 0..100 {
            let name = format!("func_{}", j);
            state.register_func(&name, |_| 0).unwrap();
        }
        drop(state);
    }

    // Verify we can still create new states
    let mut state = State::new().unwrap();
    state
        .register_func("test", |s| {
            s.push_number(42.0);
            1
        })
        .unwrap();
}

#[test]
fn test_memory_gc_interaction() {
    let mut state = State::new().unwrap();
    state.open_libs();

    state
        .register_func("create_tables", |s| {
            s.check_stack(100);
            for _ in 0..100 {
                s.new_table();
                s.pop(1);
            }
            0
        })
        .unwrap();

    let initial_mem = state.memory_usage();

    // Call many times with GC
    for i in 0..100 {
        state.do_string("create_tables()").unwrap();
        if i % 10 == 0 {
            state.gc();
        }
    }

    let final_mem = state.memory_usage();

    // Memory should not grow unboundedly
    assert!(final_mem < initial_mem * 3, "Memory grew too much");
}

// =============================================================================
// Concurrency Tests (using threads)
// =============================================================================

#[test]
fn test_concurrent_multi_state() {
    use std::thread;

    let handles: Vec<_> = (0..10)
        .map(|idx| {
            thread::spawn(move || {
                let mut state = State::new().unwrap();
                state.open_libs();

                // Register functions
                for j in 0..100 {
                    let name = format!("func_{}", j);
                    let val = j as f64;
                    state
                        .register_func(&name, move |s| {
                            s.push_number(val);
                            1
                        })
                        .unwrap();
                }

                // Call a function
                state.do_string("result = func_50()").unwrap();
                state.get_global("result").unwrap();
                let result = state.to_number(-1);
                assert_eq!(result, 50.0, "Thread {} got wrong result", idx);
            })
        })
        .collect();

    for handle in handles {
        handle.join().unwrap();
    }
}

#[test]
fn test_concurrent_state_creation_destruction() {
    use std::thread;

    let handles: Vec<_> = (0..20)
        .map(|_| {
            thread::spawn(|| {
                for _ in 0..50 {
                    let mut state = State::new().unwrap();
                    state
                        .register_func("test", |s| {
                            s.push_number(42.0);
                            1
                        })
                        .unwrap();
                    drop(state);
                }
            })
        })
        .collect();

    for handle in handles {
        handle.join().unwrap();
    }
}

#[test]
fn test_concurrent_registry_access() {
    use std::sync::atomic::{AtomicU64, Ordering};
    use std::sync::Arc;
    use std::thread;

    let total_calls = Arc::new(AtomicU64::new(0));

    let handles: Vec<_> = (0..10)
        .map(|_| {
            let total_calls = total_calls.clone();
            thread::spawn(move || {
                let mut state = State::new().unwrap();
                state.open_libs();

                let counter = total_calls.clone();
                state
                    .register_func("increment", move |s| {
                        counter.fetch_add(1, Ordering::SeqCst);
                        s.push_number(1.0);
                        1
                    })
                    .unwrap();

                // Call 1000 times
                for _ in 0..1000 {
                    state.do_string("increment()").unwrap();
                }
            })
        })
        .collect();

    for handle in handles {
        handle.join().unwrap();
    }

    // 10 threads * 1000 calls = 10000
    assert_eq!(total_calls.load(Ordering::SeqCst), 10000);
}

// =============================================================================
// Edge Case Tests
// =============================================================================

#[test]
fn test_register_func_empty_name() {
    let mut state = State::new().unwrap();

    // Empty name - just verify it doesn't crash
    let _ = state.register_func("", |s| {
        s.push_number(42.0);
        1
    });
}

#[test]
fn test_register_func_special_chars_name() {
    let mut state = State::new().unwrap();
    state.open_libs();

    // Underscore is valid
    state
        .register_func("_my_func", |s| {
            s.push_number(1.0);
            1
        })
        .unwrap();

    state.do_string("result = _my_func()").unwrap();
    state.get_global("result").unwrap();
    assert_eq!(state.to_number(-1), 1.0);
}

#[test]
fn test_register_func_unicode_name() {
    let mut state = State::new().unwrap();
    state.open_libs();

    // Register with Unicode name
    state
        .register_func("计算", |s| {
            let a = s.to_number(1);
            let b = s.to_number(2);
            s.push_number(a + b);
            1
        })
        .unwrap();

    // Access via _G["计算"]
    state.do_string(r#"result = _G["计算"](10, 20)"#).unwrap();
    state.get_global("result").unwrap();
    assert_eq!(state.to_number(-1), 30.0);
}

#[test]
fn test_register_func_call_unregistered() {
    let state = State::new().unwrap();
    state.open_libs();

    // Calling unregistered function should error
    let result = state.do_string("result = nonExistentFunc()");
    assert!(result.is_err());
}

#[test]
fn test_register_func_deep_recursion() {
    let mut state = State::new().unwrap();
    state.open_libs();

    state
        .register_func("countdown", |s| {
            let n = s.to_number(1) as i32;
            if n <= 0 {
                s.push_number(0.0);
                return 1;
            }
            // Call Luau function recursively
            s.get_global("countdown").unwrap();
            s.push_number((n - 1) as f64);
            s.pcall(1, 1).unwrap();
            let result = s.to_number(-1);
            s.pop(1);
            s.push_number(result + 1.0);
            1
        })
        .unwrap();

    // Test with moderate depth
    state.do_string("result = countdown(100)").unwrap();
    state.get_global("result").unwrap();
    assert_eq!(state.to_number(-1), 100.0);
}

// =============================================================================
// Thread/Coroutine API Tests
// =============================================================================

#[test]
fn test_new_thread() {
    let state = State::new().unwrap();
    state.open_libs();

    let thread = state.new_thread();
    assert!(thread.is_ok());

    let thread = thread.unwrap();
    assert_eq!(thread.get_top(), 0);
}

#[test]
fn test_thread_status() {
    let state = State::new().unwrap();
    state.open_libs();

    let thread = state.new_thread().unwrap();

    // New thread should be in OK status
    assert_eq!(thread.status(), CoStatus::Ok);
}

#[test]
fn test_thread_resume_simple() {
    let state = State::new().unwrap();
    state.open_libs();

    // Compile a simple function
    let bytecode = state.compile("return 42", OptLevel::O2).unwrap();

    // Create thread and load bytecode
    let thread = state.new_thread().unwrap();
    thread.load_bytecode(&bytecode, "chunk").unwrap();

    // Resume - should return Ok with 1 result
    let (status, nresults) = thread.resume(0);
    assert_eq!(status, CoStatus::Ok);
    assert_eq!(nresults, 1);
    assert_eq!(thread.to_number(-1), 42.0);
}

#[test]
fn test_thread_resume_with_args() {
    let state = State::new().unwrap();
    state.open_libs();

    // Function that adds two numbers
    let bytecode = state
        .compile(
            r#"
        local a, b = ...
        return a + b
    "#,
            OptLevel::O2,
        )
        .unwrap();

    let thread = state.new_thread().unwrap();
    thread.load_bytecode(&bytecode, "chunk").unwrap();

    // Push arguments before resume
    thread.push_number(10.0);
    thread.push_number(20.0);

    let (status, nresults) = thread.resume(2);
    assert_eq!(status, CoStatus::Ok);
    assert_eq!(nresults, 1);
    assert_eq!(thread.to_number(-1), 30.0);
}

#[test]
fn test_thread_yield_resume() {
    let state = State::new().unwrap();
    state.open_libs();

    // Function that yields multiple times
    let bytecode = state
        .compile(
            r#"
        coroutine.yield(1)
        coroutine.yield(2)
        coroutine.yield(3)
        return "done"
    "#,
            OptLevel::O2,
        )
        .unwrap();

    let thread = state.new_thread().unwrap();
    thread.load_bytecode(&bytecode, "chunk").unwrap();

    // First resume - should yield 1
    let (status, nresults) = thread.resume(0);
    assert_eq!(status, CoStatus::Yield);
    assert_eq!(nresults, 1);
    assert_eq!(thread.to_number(-1), 1.0);
    thread.pop(nresults);

    // Second resume - should yield 2
    let (status, nresults) = thread.resume(0);
    assert_eq!(status, CoStatus::Yield);
    assert_eq!(nresults, 1);
    assert_eq!(thread.to_number(-1), 2.0);
    thread.pop(nresults);

    // Third resume - should yield 3
    let (status, nresults) = thread.resume(0);
    assert_eq!(status, CoStatus::Yield);
    assert_eq!(nresults, 1);
    assert_eq!(thread.to_number(-1), 3.0);
    thread.pop(nresults);

    // Fourth resume - should return "done"
    let (status, nresults) = thread.resume(0);
    assert_eq!(status, CoStatus::Ok);
    assert_eq!(nresults, 1);
    assert_eq!(thread.to_string(-1), Some("done".to_string()));
}

#[test]
fn test_thread_is_yieldable() {
    let state = State::new().unwrap();
    state.open_libs();

    // Note: In Luau, even the main state might be considered "yieldable" in some contexts,
    // so we don't assert on that. We just test the thread functionality.

    // Compile function that yields
    let bytecode = state
        .compile(
            r#"
        -- Inside coroutine, yield
        coroutine.yield("yieldable")
        return "done"
    "#,
            OptLevel::O2,
        )
        .unwrap();

    let thread = state.new_thread().unwrap();
    thread.load_bytecode(&bytecode, "chunk").unwrap();

    // Resume to first yield
    let (status, nresults) = thread.resume(0);
    assert_eq!(status, CoStatus::Yield);
    assert_eq!(nresults, 1);
    assert_eq!(thread.to_string(-1), Some("yieldable".to_string()));
}

#[test]
fn test_thread_multiple_returns() {
    let state = State::new().unwrap();
    state.open_libs();

    let bytecode = state
        .compile(
            r#"
        return 1, 2, 3, 4, 5
    "#,
            OptLevel::O2,
        )
        .unwrap();

    let thread = state.new_thread().unwrap();
    thread.load_bytecode(&bytecode, "chunk").unwrap();

    let (status, nresults) = thread.resume(0);
    assert_eq!(status, CoStatus::Ok);
    assert_eq!(nresults, 5);

    // Check all return values (they're in order on the stack)
    assert_eq!(thread.to_number(-5), 1.0);
    assert_eq!(thread.to_number(-4), 2.0);
    assert_eq!(thread.to_number(-3), 3.0);
    assert_eq!(thread.to_number(-2), 4.0);
    assert_eq!(thread.to_number(-1), 5.0);
}

#[test]
fn test_thread_error() {
    let state = State::new().unwrap();
    state.open_libs();

    let bytecode = state
        .compile(
            r#"
        error("test error")
    "#,
            OptLevel::O2,
        )
        .unwrap();

    let thread = state.new_thread().unwrap();
    thread.load_bytecode(&bytecode, "chunk").unwrap();

    let (status, nresults) = thread.resume(0);
    assert_eq!(status, CoStatus::ErrRun);
    // When a coroutine errors, results are pushed onto the stack
    // The number of results may vary (error message, plus possibly debug info)
    assert!(nresults >= 1, "at least error message should be on stack");
    // The top of the stack should contain the error message
    let err_msg = thread.to_string(-1);
    assert!(err_msg.is_some(), "error message should be on stack");
    assert!(
        err_msg.unwrap().contains("test error"),
        "error message should contain 'test error'"
    );
}

#[test]
fn test_thread_pass_values_on_resume() {
    let state = State::new().unwrap();
    state.open_libs();

    // Function that receives values via yield
    let bytecode = state
        .compile(
            r#"
        local x = coroutine.yield()  -- Receive value here
        return x * 2
    "#,
            OptLevel::O2,
        )
        .unwrap();

    let thread = state.new_thread().unwrap();
    thread.load_bytecode(&bytecode, "chunk").unwrap();

    // First resume - starts coroutine, yields
    let (status, _) = thread.resume(0);
    assert_eq!(status, CoStatus::Yield);

    // Second resume - pass value 21, should return 42
    thread.push_number(21.0);
    let (status, nresults) = thread.resume(1);
    assert_eq!(status, CoStatus::Ok);
    assert_eq!(nresults, 1);
    assert_eq!(thread.to_number(-1), 42.0);
}

#[test]
fn test_costatus_display() {
    assert_eq!(format!("{}", CoStatus::Ok), "ok");
    assert_eq!(format!("{}", CoStatus::Yield), "yield");
    assert_eq!(format!("{}", CoStatus::ErrRun), "errrun");
    assert_eq!(format!("{}", CoStatus::ErrSyntax), "errsyntax");
    assert_eq!(format!("{}", CoStatus::ErrMem), "errmem");
    assert_eq!(format!("{}", CoStatus::ErrErr), "errerr");
    assert_eq!(format!("{}", CoStatus::Break), "break");
}

#[test]
fn test_thread_stack_operations() {
    let state = State::new().unwrap();
    state.open_libs();

    let thread = state.new_thread().unwrap();

    // Test basic stack operations on thread
    thread.push_number(1.0);
    thread.push_number(2.0);
    thread.push_string("hello").unwrap();
    thread.push_boolean(true);
    thread.push_nil();

    assert_eq!(thread.get_top(), 5);
    assert!(thread.is_nil(-1));
    assert!(thread.is_boolean(-2));
    assert!(thread.is_string(-3));
    assert!(thread.is_number(-4));
    assert!(thread.is_number(-5));

    assert!(thread.to_boolean(-2));
    assert_eq!(thread.to_string(-3), Some("hello".to_string()));
    assert_eq!(thread.to_number(-4), 2.0);
    assert_eq!(thread.to_number(-5), 1.0);

    thread.pop(3);
    assert_eq!(thread.get_top(), 2);
}

#[test]
fn test_thread_table_operations() {
    let state = State::new().unwrap();
    state.open_libs();

    let thread = state.new_thread().unwrap();

    // Create and manipulate table on thread stack
    thread.new_table();
    thread.push_number(42.0);
    thread.set_field(-2, "answer").unwrap();

    thread.get_field(-1, "answer").unwrap();
    assert_eq!(thread.to_number(-1), 42.0);
}

#[test]
fn test_thread_lifecycle_correct_order() {
    // This test demonstrates the correct lifecycle: Thread dropped before State.
    // This pattern must be followed to avoid undefined behavior.
    let state = State::new().unwrap();
    state.open_libs();

    let bytecode = state
        .compile("return 'hello from thread'", OptLevel::O2)
        .unwrap();

    // Create thread and use it
    {
        let thread = state.new_thread().unwrap();
        thread.load_bytecode(&bytecode, "chunk").unwrap();

        let (status, nresults) = thread.resume(0);
        assert_eq!(status, CoStatus::Ok);
        assert_eq!(nresults, 1);
        assert_eq!(thread.to_string(-1), Some("hello from thread".to_string()));

        // Thread is dropped here at end of scope - BEFORE state
    }

    // State can still be used after thread is dropped
    state.push_string("state still works").unwrap();
    assert_eq!(state.to_string(-1), Some("state still works".to_string()));

    // State is dropped here
}

#[test]
fn test_multiple_threads_lifecycle() {
    // Test that multiple threads can be created and dropped properly
    let state = State::new().unwrap();
    state.open_libs();

    let bytecode1 = state.compile("return 1", OptLevel::O2).unwrap();
    let bytecode2 = state.compile("return 2", OptLevel::O2).unwrap();

    // Create multiple threads
    let thread1 = state.new_thread().unwrap();
    let thread2 = state.new_thread().unwrap();

    thread1.load_bytecode(&bytecode1, "chunk1").unwrap();
    thread2.load_bytecode(&bytecode2, "chunk2").unwrap();

    let (status1, _) = thread1.resume(0);
    let (status2, _) = thread2.resume(0);

    assert_eq!(status1, CoStatus::Ok);
    assert_eq!(status2, CoStatus::Ok);
    assert_eq!(thread1.to_number(-1), 1.0);
    assert_eq!(thread2.to_number(-1), 2.0);

    // Both threads dropped before state (at end of function)
}
