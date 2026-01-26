//! Luau benchmarks using the actual giztoy_luau API.

use criterion::{black_box, criterion_group, criterion_main, Criterion, Throughput};
use giztoy_luau::{OptLevel, State};

// =============================================================================
// Script Execution Benchmarks
// =============================================================================

fn bench_do_string_simple(c: &mut Criterion) {
    let state = State::new().unwrap();

    c.bench_function("do_string_simple", |b| {
        b.iter(|| {
            state.do_string(black_box("x = 1 + 2")).unwrap();
        })
    });
}

fn bench_do_string_arithmetic(c: &mut Criterion) {
    let state = State::new().unwrap();
    let script = r#"
        local a = 0
        for i = 1, 100 do
            a = a + i * 2 - 1
        end
    "#;

    c.bench_function("do_string_arithmetic", |b| {
        b.iter(|| {
            state.do_string(black_box(script)).unwrap();
        })
    });
}

fn bench_do_string_fibonacci(c: &mut Criterion) {
    let state = State::new().unwrap();
    let script = r#"
        local function fib(n)
            if n <= 1 then return n end
            return fib(n - 1) + fib(n - 2)
        end
        local result = fib(15)
    "#;

    c.bench_function("do_string_fibonacci", |b| {
        b.iter(|| {
            state.do_string(black_box(script)).unwrap();
        })
    });
}

fn bench_do_string_loop_10k(c: &mut Criterion) {
    let state = State::new().unwrap();
    let script = r#"
        local sum = 0
        for i = 1, 10000 do
            sum = sum + i
        end
    "#;

    c.bench_function("do_string_loop_10k", |b| {
        b.iter(|| {
            state.do_string(black_box(script)).unwrap();
        })
    });
}

fn bench_do_string_string_ops(c: &mut Criterion) {
    let state = State::new().unwrap();
    state.open_libs();
    let script = r#"
        local s = ""
        for i = 1, 100 do
            s = s .. tostring(i)
        end
    "#;

    c.bench_function("do_string_string_ops", |b| {
        b.iter(|| {
            state.do_string(black_box(script)).unwrap();
        })
    });
}

fn bench_do_string_table_ops(c: &mut Criterion) {
    let state = State::new().unwrap();
    let script = r#"
        local t = {}
        for i = 1, 1000 do
            t[i] = i * 2
        end
        local sum = 0
        for i = 1, 1000 do
            sum = sum + t[i]
        end
    "#;

    c.bench_function("do_string_table_ops", |b| {
        b.iter(|| {
            state.do_string(black_box(script)).unwrap();
        })
    });
}

// =============================================================================
// Compilation Benchmarks
// =============================================================================

fn bench_compile_simple(c: &mut Criterion) {
    let state = State::new().unwrap();

    c.bench_function("compile_simple", |b| {
        b.iter(|| {
            let _ = state.compile(black_box("x = 1 + 2"), OptLevel::O2).unwrap();
        })
    });
}

fn bench_compile_complex(c: &mut Criterion) {
    let state = State::new().unwrap();
    let script = r#"
        local function fib(n)
            if n <= 1 then return n end
            return fib(n - 1) + fib(n - 2)
        end
        return fib(20)
    "#;

    c.bench_function("compile_complex", |b| {
        b.iter(|| {
            let _ = state.compile(black_box(script), OptLevel::O2).unwrap();
        })
    });
}

fn bench_load_bytecode(c: &mut Criterion) {
    let state = State::new().unwrap();
    let bytecode = state.compile("x = 42", OptLevel::O2).unwrap();

    c.bench_function("load_bytecode", |b| {
        b.iter(|| {
            state.load_bytecode(black_box(&bytecode), "chunk").unwrap();
            state.pop(1); // Remove the function from stack
        })
    });
}

// =============================================================================
// Stack Operation Benchmarks
// =============================================================================

fn bench_push_pop_number(c: &mut Criterion) {
    let state = State::new().unwrap();

    c.bench_function("push_pop_number", |b| {
        b.iter(|| {
            state.push_number(black_box(42.0));
            black_box(state.to_number(-1));
            state.pop(1);
        })
    });
}

fn bench_push_pop_string(c: &mut Criterion) {
    let state = State::new().unwrap();

    c.bench_function("push_pop_string", |b| {
        b.iter(|| {
            state.push_string(black_box("hello world")).unwrap();
            black_box(state.to_string(-1));
            state.pop(1);
        })
    });
}

fn bench_push_pop_boolean(c: &mut Criterion) {
    let state = State::new().unwrap();

    c.bench_function("push_pop_boolean", |b| {
        b.iter(|| {
            state.push_boolean(black_box(true));
            black_box(state.to_boolean(-1));
            state.pop(1);
        })
    });
}

// =============================================================================
// Global Variable Benchmarks
// =============================================================================

fn bench_set_global(c: &mut Criterion) {
    let state = State::new().unwrap();

    c.bench_function("set_global", |b| {
        b.iter(|| {
            state.push_number(black_box(42.0));
            state.set_global(black_box("x")).unwrap();
        })
    });
}

fn bench_get_global(c: &mut Criterion) {
    let state = State::new().unwrap();
    state.push_number(42.0);
    state.set_global("x").unwrap();

    c.bench_function("get_global", |b| {
        b.iter(|| {
            state.get_global(black_box("x")).unwrap();
            black_box(state.to_number(-1));
            state.pop(1);
        })
    });
}

// =============================================================================
// Table Benchmarks
// =============================================================================

fn bench_create_table(c: &mut Criterion) {
    let state = State::new().unwrap();

    c.bench_function("create_table", |b| {
        b.iter(|| {
            state.new_table();
            state.pop(1);
        })
    });
}

fn bench_table_set_field(c: &mut Criterion) {
    let state = State::new().unwrap();
    state.new_table();

    c.bench_function("table_set_field", |b| {
        b.iter(|| {
            state.push_number(black_box(42.0));
            state.set_field(-2, black_box("key")).unwrap();
        })
    });
}

fn bench_table_get_field(c: &mut Criterion) {
    let state = State::new().unwrap();
    state.new_table();
    state.push_number(42.0);
    state.set_field(-2, "key").unwrap();

    c.bench_function("table_get_field", |b| {
        b.iter(|| {
            state.get_field(-1, black_box("key")).unwrap();
            black_box(state.to_number(-1));
            state.pop(1);
        })
    });
}

// =============================================================================
// Memory Benchmarks
// =============================================================================

fn bench_memory_usage(c: &mut Criterion) {
    let state = State::new().unwrap();

    c.bench_function("memory_usage", |b| {
        b.iter(|| {
            black_box(state.memory_usage());
        })
    });
}

fn bench_gc(c: &mut Criterion) {
    let state = State::new().unwrap();
    state.open_libs();
    // Allocate some memory first
    state
        .do_string(
            r#"
        for _ = 1, 100 do
            local t = {}
            for i = 1, 100 do
                t[i] = tostring(i)
            end
        end
    "#,
        )
        .unwrap();

    c.bench_function("gc", |b| {
        b.iter(|| {
            state.gc();
        })
    });
}

fn bench_memory_allocation(c: &mut Criterion) {
    let state = State::new().unwrap();
    let script = r#"
        local t = {}
        for i = 1, 1000 do
            t[i] = {x = i, y = i * 2, z = tostring(i)}
        end
    "#;

    c.bench_function("memory_allocation", |b| {
        b.iter(|| {
            state.do_string(black_box(script)).unwrap();
            state.gc();
        })
    });
}

// =============================================================================
// State Creation Benchmarks
// =============================================================================

fn bench_new_close(c: &mut Criterion) {
    c.bench_function("new_close", |b| {
        b.iter(|| {
            let state = State::new().unwrap();
            drop(state);
        })
    });
}

fn bench_new_with_libs(c: &mut Criterion) {
    c.bench_function("new_with_libs", |b| {
        b.iter(|| {
            let state = State::new().unwrap();
            state.open_libs();
            drop(state);
        })
    });
}

// =============================================================================
// Real-World Scenario Benchmarks
// =============================================================================

fn bench_json_like_processing(c: &mut Criterion) {
    let state = State::new().unwrap();
    state.open_libs();
    let script = r#"
        local data = {
            users = {},
            total = 0
        }
        
        for i = 1, 100 do
            data.users[i] = {
                id = i,
                name = "user" .. i,
                email = "user" .. i .. "@example.com",
                active = i % 2 == 0
            }
            if data.users[i].active then
                data.total = data.total + 1
            end
        end
        
        result = data.total
    "#;

    c.bench_function("json_like_processing", |b| {
        b.iter(|| {
            state.do_string(black_box(script)).unwrap();
        })
    });
}

fn bench_config_parsing(c: &mut Criterion) {
    let state = State::new().unwrap();
    state.open_libs();
    let script = r#"
        local config = {
            server = {
                host = "localhost",
                port = 8080,
                timeout = 30
            },
            database = {
                driver = "postgres",
                host = "db.example.com",
                port = 5432,
                name = "mydb"
            },
            features = {
                auth = true,
                cache = true,
                logging = {
                    level = "info",
                    format = "json"
                }
            }
        }
        
        result = config.server.host .. ":" .. tostring(config.server.port)
        if config.features.auth then
            result = result .. " (auth enabled)"
        end
    "#;

    c.bench_function("config_parsing", |b| {
        b.iter(|| {
            state.do_string(black_box(script)).unwrap();
        })
    });
}

fn bench_agent_tool_simulation(c: &mut Criterion) {
    let state = State::new().unwrap();
    state.open_libs();
    let script = r#"
        local function invoke(args)
            if not args.query then
                return {error = "missing query"}
            end
            
            local results = {}
            for i = 1, 10 do
                results[i] = {
                    id = i,
                    score = math.random(),
                    match = string.match(args.query, "^%w+")
                }
            end
            
            table.sort(results, function(a, b) return a.score > b.score end)
            
            return {
                query = args.query,
                count = #results,
                results = results
            }
        end
        
        local result = invoke({query = "test query string"})
        final_count = result.count
    "#;

    c.bench_function("agent_tool_simulation", |b| {
        b.iter(|| {
            state.do_string(black_box(script)).unwrap();
        })
    });
}

// =============================================================================
// Throughput Benchmarks
// =============================================================================

fn bench_script_throughput(c: &mut Criterion) {
    let state = State::new().unwrap();
    let script = "local x = 1 + 2";

    let mut group = c.benchmark_group("script_throughput");
    group.throughput(Throughput::Elements(1));
    group.bench_function("simple_script", |b| {
        b.iter(|| {
            state.do_string(black_box(script)).unwrap();
        })
    });
    group.finish();
}

fn bench_iteration_throughput(c: &mut Criterion) {
    let state = State::new().unwrap();
    let script = r#"
        local sum = 0
        for i = 1, 1000 do
            sum = sum + i
        end
    "#;

    let mut group = c.benchmark_group("iteration_throughput");
    group.throughput(Throughput::Elements(1000));
    group.bench_function("1000_iterations", |b| {
        b.iter(|| {
            state.do_string(black_box(script)).unwrap();
        })
    });
    group.finish();
}

// =============================================================================
// Criterion Groups
// =============================================================================

criterion_group!(
    script_execution,
    bench_do_string_simple,
    bench_do_string_arithmetic,
    bench_do_string_fibonacci,
    bench_do_string_loop_10k,
    bench_do_string_string_ops,
    bench_do_string_table_ops,
);

criterion_group!(
    compilation,
    bench_compile_simple,
    bench_compile_complex,
    bench_load_bytecode,
);

criterion_group!(
    stack_ops,
    bench_push_pop_number,
    bench_push_pop_string,
    bench_push_pop_boolean,
);

criterion_group!(globals, bench_set_global, bench_get_global,);

criterion_group!(
    tables,
    bench_create_table,
    bench_table_set_field,
    bench_table_get_field,
);

criterion_group!(memory, bench_memory_usage, bench_gc, bench_memory_allocation,);

criterion_group!(state, bench_new_close, bench_new_with_libs,);

criterion_group!(
    realworld,
    bench_json_like_processing,
    bench_config_parsing,
    bench_agent_tool_simulation,
);

criterion_group!(
    throughput,
    bench_script_throughput,
    bench_iteration_throughput,
);

criterion_main!(
    script_execution,
    compilation,
    stack_ops,
    globals,
    tables,
    memory,
    state,
    realworld,
    throughput,
);
