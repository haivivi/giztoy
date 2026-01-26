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
