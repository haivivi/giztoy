//! Function Tool example - Demonstrating tool calling with GenX.
//!
//! This example shows how to define function tools and let the AI
//! call them to answer user questions.

use anyhow::{Context, Result};
use giztoy_genx::openai::{OpenAIConfig, OpenAIGenerator};
use giztoy_genx::{stream::collect_text, FuncTool, Generator, ModelContextBuilder};
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};
use std::env;
use tracing::{info, Level};
use tracing_subscriber::FmtSubscriber;

// ============================================================================
// Tool Argument Types (with JsonSchema for automatic schema generation)
// ============================================================================

/// Arguments for the weather tool
#[derive(Debug, Clone, JsonSchema, Serialize, Deserialize)]
struct WeatherArgs {
    /// City name to get weather for
    #[serde(alias = "location", alias = "place", alias = "城市")]
    city: String,
    /// Temperature unit (celsius or fahrenheit)
    #[serde(default = "default_unit")]
    unit: String,
}

fn default_unit() -> String {
    "celsius".to_string()
}

/// Arguments for the news tool
#[derive(Debug, Clone, JsonSchema, Serialize, Deserialize)]
struct NewsArgs {
    /// Topic to search news for (e.g., "科技", "体育", "财经", "娱乐")
    #[serde(alias = "category")]
    topic: String,
    /// Number of articles to return (1-10)
    #[serde(default = "default_count")]
    count: u32,
}

fn default_count() -> u32 {
    3
}

/// Arguments for the calculator tool
#[derive(Debug, Clone, JsonSchema, Serialize, Deserialize)]
struct CalculatorArgs {
    /// Mathematical expression to evaluate
    expression: String,
}

/// Arguments for the translation tool
#[derive(Debug, Clone, JsonSchema, Serialize, Deserialize)]
struct TranslateArgs {
    /// Text to translate
    text: String,
    /// Target language (e.g., "english", "chinese", "japanese")
    target_language: String,
}

/// Arguments for stock price tool
#[derive(Debug, Clone, JsonSchema, Serialize, Deserialize)]
struct StockPriceArgs {
    /// Stock symbol (e.g., "AAPL", "GOOGL")
    symbol: String,
}

// ============================================================================
// Fake Tool Implementations (simulating real API responses)
// ============================================================================

fn get_weather(args: &WeatherArgs) -> String {
    // Fake weather data
    let weather_data = vec![
        ("北京", 15, "多云", 45),
        ("上海", 18, "晴", 60),
        ("广州", 25, "阴天", 80),
        ("深圳", 26, "小雨", 85),
        ("杭州", 17, "晴", 55),
        ("成都", 14, "多云", 70),
        ("New York", 12, "Sunny", 40),
        ("London", 8, "Rainy", 75),
        ("Tokyo", 16, "Cloudy", 50),
        ("Paris", 10, "Partly cloudy", 65),
    ];

    let city_lower = args.city.to_lowercase();
    for (city, temp_c, condition, humidity) in &weather_data {
        if city.to_lowercase().contains(&city_lower) || city_lower.contains(&city.to_lowercase()) {
            let temp = if args.unit == "fahrenheit" {
                format!("{}°F", temp_c * 9 / 5 + 32)
            } else {
                format!("{}°C", temp_c)
            };
            return serde_json::json!({
                "city": city,
                "temperature": temp,
                "condition": condition,
                "humidity": format!("{}%", humidity),
                "wind": "10 km/h",
                "updated_at": "2026-01-21 12:00:00"
            })
            .to_string();
        }
    }

    serde_json::json!({
        "error": "City not found",
        "suggestion": "Try: 北京, 上海, 广州, 深圳, 杭州, 成都, New York, London, Tokyo, Paris"
    })
    .to_string()
}

fn get_news(args: &NewsArgs) -> String {
    // Fake news data by topic
    let all_news = vec![
        ("科技", vec![
            ("苹果发布新一代 Vision Pro 2，支持 8K 显示", "Apple Today", "2小时前"),
            ("OpenAI 推出 GPT-5，性能提升300%", "Tech Crunch", "4小时前"),
            ("特斯拉全自动驾驶获得欧盟认证", "Reuters", "6小时前"),
            ("微软宣布 Windows 12 将于下月发布", "The Verge", "8小时前"),
            ("谷歌量子计算机实现新突破", "Wired", "10小时前"),
        ]),
        ("体育", vec![
            ("梅西带领迈阿密国际获得美国杯冠军", "ESPN", "1小时前"),
            ("NBA季后赛：湖人4-2击败凯尔特人", "Sports Illustrated", "3小时前"),
            ("中国女足晋级世界杯八强", "新华社", "5小时前"),
            ("F1中国大奖赛：汉密尔顿夺冠", "Autosport", "7小时前"),
        ]),
        ("财经", vec![
            ("比特币突破10万美元大关", "Bloomberg", "2小时前"),
            ("美联储宣布维持利率不变", "WSJ", "4小时前"),
            ("阿里巴巴股价创年内新高", "财新", "6小时前"),
            ("黄金价格连续第五天上涨", "Reuters", "8小时前"),
        ]),
        ("娱乐", vec![
            ("《流浪地球3》定档春节", "猫眼", "1小时前"),
            ("Taylor Swift 宣布亚洲巡演计划", "Billboard", "3小时前"),
            ("Netflix 原创剧《鱿鱼游戏3》开播", "Variety", "5小时前"),
        ]),
    ];

    let topic_lower = args.topic.to_lowercase();
    let mut results = Vec::new();

    for (category, news_list) in &all_news {
        if category.to_lowercase().contains(&topic_lower)
            || topic_lower.contains(&category.to_lowercase())
            || topic_lower.contains("全部")
            || topic_lower.contains("all")
        {
            for (i, (title, source, time)) in news_list.iter().enumerate() {
                if i >= args.count as usize {
                    break;
                }
                results.push(serde_json::json!({
                    "title": title,
                    "source": source,
                    "time": time,
                    "category": category
                }));
            }
        }
    }

    if results.is_empty() {
        // Return general news if no match
        results.push(serde_json::json!({
            "title": "今日热点：全球科技股普涨",
            "source": "综合",
            "time": "刚刚",
            "category": "综合"
        }));
    }

    serde_json::json!({
        "topic": args.topic,
        "count": results.len(),
        "articles": results
    })
    .to_string()
}

fn calculate(args: &CalculatorArgs) -> String {
    // Simple expression evaluator (fake implementation)
    let expr = args.expression.trim();

    // Helper to parse operands
    fn parse_binary_op(expr: &str, op: char) -> Option<f64> {
        let pos = if op == '-' {
            // For minus, find the rightmost one (to handle negative numbers)
            expr.rfind(op).filter(|&p| p > 0)
        } else {
            expr.find(op)
        }?;

        let (a, b) = expr.split_at(pos);
        let a: f64 = a.trim().parse().ok()?;
        let b: f64 = b[1..].trim().parse().ok()?;

        match op {
            '+' => Some(a + b),
            '-' => Some(a - b),
            '*' => Some(a * b),
            '/' if b != 0.0 => Some(a / b),
            _ => None,
        }
    }

    // Try each operator in order of precedence (simple left-to-right)
    let result = parse_binary_op(expr, '+')
        .or_else(|| parse_binary_op(expr, '-'))
        .or_else(|| parse_binary_op(expr, '*'))
        .or_else(|| parse_binary_op(expr, '/'));

    match result {
        Some(r) => serde_json::json!({
            "expression": expr,
            "result": r,
            "formatted": format!("{} = {}", expr, r)
        })
        .to_string(),
        None => serde_json::json!({
            "expression": expr,
            "error": "无法计算该表达式",
            "hint": "支持的运算符：+, -, *, /"
        })
        .to_string(),
    }
}

fn translate(args: &TranslateArgs) -> String {
    // Fake translation (just returns a mock response)
    let lang = args.target_language.to_lowercase();
    let translated = match lang.as_str() {
        "chinese" | "中文" | "zh" | "zh-cn" | "zh-hans" => {
            // Fake "translation" by adding Chinese greeting
            format!("你好，世界！（原文：{}）", args.text)
        }
        "english" | "英语" | "英文" | "en" => {
            format!("Hello, World! (original: {})", args.text)
        }
        "japanese" | "日语" | "日文" | "ja" | "jp" => {
            format!("こんにちは、世界！（原文：{}）", args.text)
        }
        "korean" | "韩语" | "韩文" | "ko" | "kr" => {
            format!("안녕하세요, 세상! (원문: {})", args.text)
        }
        "french" | "法语" | "法文" | "fr" => {
            format!("Bonjour le monde! (original: {})", args.text)
        }
        "spanish" | "西班牙语" | "es" => {
            format!("¡Hola, Mundo! (original: {})", args.text)
        }
        "german" | "德语" | "de" => {
            format!("Hallo, Welt! (original: {})", args.text)
        }
        _ => format!("[{} translation] {}", args.target_language, args.text),
    };

    serde_json::json!({
        "original": args.text,
        "target_language": args.target_language,
        "translated": translated
    })
    .to_string()
}

fn get_stock_price(args: &StockPriceArgs) -> String {
    // Fake stock data
    let stocks = vec![
        ("AAPL", "Apple Inc.", 185.50, 2.3, 1.26),
        ("GOOGL", "Alphabet Inc.", 142.80, -1.2, -0.83),
        ("MSFT", "Microsoft Corp.", 378.90, 3.1, 0.82),
        ("AMZN", "Amazon.com Inc.", 178.25, 1.8, 1.02),
        ("TSLA", "Tesla Inc.", 248.60, -4.5, -1.78),
        ("META", "Meta Platforms", 485.30, 5.2, 1.08),
        ("NVDA", "NVIDIA Corp.", 875.20, 8.5, 0.98),
        ("BABA", "Alibaba Group", 85.40, 1.2, 1.43),
        ("JD", "JD.com Inc.", 28.90, 0.8, 2.85),
        ("BIDU", "Baidu Inc.", 105.60, -0.5, -0.47),
    ];

    let symbol_upper = args.symbol.to_uppercase();
    for (symbol, name, price, change, change_pct) in &stocks {
        if *symbol == symbol_upper {
            return serde_json::json!({
                "symbol": symbol,
                "name": name,
                "price": format!("${:.2}", price),
                "change": format!("{:+.2}", change),
                "change_percent": format!("{:+.2}%", change_pct),
                "market_cap": "万亿美元级",
                "updated_at": "2026-01-21 15:30:00 EST"
            })
            .to_string();
        }
    }

    serde_json::json!({
        "error": "Stock symbol not found",
        "symbol": args.symbol,
        "suggestion": "Try: AAPL, GOOGL, MSFT, AMZN, TSLA, META, NVDA, BABA, JD, BIDU"
    })
    .to_string()
}

// ============================================================================
// Tool Execution
// ============================================================================

fn execute_tool(name: &str, arguments: &str) -> Result<String> {
    // Try to fix common JSON issues
    let args_str = arguments.trim();

    // Helper to extract string from JSON value
    fn get_string(v: &serde_json::Value, keys: &[&str]) -> Option<String> {
        for key in keys {
            if let Some(val) = v.get(*key).and_then(|v| v.as_str()) {
                return Some(val.to_string());
            }
        }
        None
    }

    match name {
        "get_weather" => {
            // Flexible parsing for weather
            let args: WeatherArgs = serde_json::from_str(args_str)
                .or_else(|_| {
                    let v: serde_json::Value = serde_json::from_str(args_str)?;
                    let city = get_string(&v, &["city", "location", "place", "城市"])
                        .unwrap_or_else(|| "北京".to_string());
                    let unit = get_string(&v, &["unit", "单位"])
                        .unwrap_or_else(|| "celsius".to_string());
                    Ok::<_, serde_json::Error>(WeatherArgs { city, unit })
                })
                .map_err(|e| anyhow::anyhow!("Weather args parse error: {} (input: {})", e, args_str))?;
            Ok(get_weather(&args))
        }
        "get_news" => {
            // Flexible parsing for news
            let args: NewsArgs = serde_json::from_str(args_str)
                .or_else(|_| {
                    let v: serde_json::Value = serde_json::from_str(args_str)?;
                    let topic = get_string(&v, &["topic", "category", "query", "话题"])
                        .unwrap_or_else(|| "科技".to_string());
                    let count = v.get("count")
                        .and_then(|v| v.as_u64())
                        .unwrap_or(3) as u32;
                    Ok::<_, serde_json::Error>(NewsArgs { topic, count })
                })
                .map_err(|e| anyhow::anyhow!("News args parse error: {} (input: {})", e, args_str))?;
            Ok(get_news(&args))
        }
        "calculate" => {
            // Flexible parsing for calculator
            let args: CalculatorArgs = serde_json::from_str(args_str)
                .or_else(|_| {
                    let v: serde_json::Value = serde_json::from_str(args_str)?;
                    let expression = get_string(&v, &["expression", "expr", "formula", "calculation", "计算式"])
                        .unwrap_or_else(|| "0".to_string());
                    Ok::<_, serde_json::Error>(CalculatorArgs { expression })
                })
                .map_err(|e| anyhow::anyhow!("Calculator args parse error: {} (input: {})", e, args_str))?;
            Ok(calculate(&args))
        }
        "translate" => {
            // Flexible parsing for translate
            let args: TranslateArgs = serde_json::from_str(args_str)
                .or_else(|_| {
                    let v: serde_json::Value = serde_json::from_str(args_str)?;
                    let text = get_string(&v, &["text", "content", "source", "原文"])
                        .unwrap_or_default();
                    let target_language = get_string(&v, &["target_language", "target", "to", "language", "目标语言"])
                        .unwrap_or_else(|| "chinese".to_string());
                    Ok::<_, serde_json::Error>(TranslateArgs { text, target_language })
                })
                .map_err(|e| anyhow::anyhow!("Translate args parse error: {} (input: {})", e, args_str))?;
            Ok(translate(&args))
        }
        "get_stock_price" => {
            // Flexible parsing for stock
            let args: StockPriceArgs = serde_json::from_str(args_str)
                .or_else(|_| {
                    let v: serde_json::Value = serde_json::from_str(args_str)?;
                    let symbol = get_string(&v, &["symbol", "ticker", "stock", "code", "股票代码"])
                        .unwrap_or_else(|| "AAPL".to_string());
                    Ok::<_, serde_json::Error>(StockPriceArgs { symbol })
                })
                .map_err(|e| anyhow::anyhow!("Stock args parse error: {} (input: {})", e, args_str))?;
            Ok(get_stock_price(&args))
        }
        _ => Ok(serde_json::json!({"error": format!("Unknown tool: {}", name)}).to_string()),
    }
}

// ============================================================================
// Main
// ============================================================================

const SYSTEM_PROMPT: &str = r#"你是一个智能助手，可以使用以下工具来回答用户的问题：

1. **get_weather** - 获取城市天气信息
2. **get_news** - 获取最新新闻（支持科技、体育、财经、娱乐等话题）
3. **calculate** - 计算数学表达式
4. **translate** - 翻译文本
5. **get_stock_price** - 获取股票价格

请根据用户的问题，决定是否需要调用工具。如果需要，请按照工具的参数格式调用。
调用工具后，请根据工具返回的结果，用自然语言回答用户的问题。"#;

#[tokio::main]
async fn main() -> Result<()> {
    // Initialize logging
    let subscriber = FmtSubscriber::builder()
        .with_max_level(Level::INFO)
        .finish();
    tracing::subscriber::set_global_default(subscriber)?;

    // Read API key from environment variable
    let openai_api_key = env::var("OPENAI_API_KEY")
        .context("OPENAI_API_KEY environment variable not set")?;

    info!("🛠️ Starting Function Tool Demo");
    println!("\n============================================================");
    println!("  🛠️ GenX Function Tool Demo");
    println!("============================================================\n");

    // Create OpenAI generator
    let generator = OpenAIGenerator::new(OpenAIConfig {
        api_key: openai_api_key,
        model: "gpt-4o-mini".to_string(),
        ..Default::default()
    });

    // Create tools
    let weather_tool = FuncTool::new::<WeatherArgs>("get_weather", "获取指定城市的天气信息");
    let news_tool = FuncTool::new::<NewsArgs>("get_news", "获取指定话题的最新新闻");
    let calc_tool = FuncTool::new::<CalculatorArgs>("calculate", "计算数学表达式");
    let translate_tool = FuncTool::new::<TranslateArgs>("translate", "将文本翻译成指定语言");
    let stock_tool = FuncTool::new::<StockPriceArgs>("get_stock_price", "获取股票实时价格");

    // Print tool schemas
    println!("📋 已注册的工具：\n");
    for tool in [&weather_tool, &news_tool, &calc_tool, &translate_tool, &stock_tool] {
        println!("  • {} - {}", tool.name, tool.description);
    }
    println!();

    // Test queries
    let queries = vec![
        "北京今天天气怎么样？",
        "帮我查一下最新的科技新闻",
        "计算一下 123 * 456",
        "把 'Hello, World!' 翻译成中文",
        "苹果公司的股票价格是多少？",
        "上海和东京的天气对比一下",
    ];

    for (i, query) in queries.iter().enumerate() {
        println!("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━");
        println!("📝 问题 {}: {}\n", i + 1, query);

        // First call: Let AI decide which tool to use
        let mut builder = ModelContextBuilder::new();
        builder.prompt_text("system", SYSTEM_PROMPT);
        builder.user_text("user", *query);

        // Add tool definitions (for context, though actual calling is simulated)
        builder.add_tool(weather_tool.clone());
        builder.add_tool(news_tool.clone());
        builder.add_tool(calc_tool.clone());
        builder.add_tool(translate_tool.clone());
        builder.add_tool(stock_tool.clone());

        // Use invoke to get structured tool call
        // For this demo, we'll use a simpler approach: ask the model to output JSON
        let invoke_prompt = format!(
            r#"用户问题：{}

请分析用户的问题，并决定需要调用哪个工具。以JSON格式输出：
{{
    "tool": "工具名称",
    "arguments": {{ 工具参数 }},
    "reasoning": "调用理由"
}}

可用工具：get_weather, get_news, calculate, translate, get_stock_price"#,
            query
        );

        builder = ModelContextBuilder::new();
        builder.prompt_text("system", "你是一个工具调用助手，请输出JSON格式的工具调用指令。");
        builder.user_text("user", &invoke_prompt);

        let ctx = builder.build();
        let mut stream = generator.generate_stream("", &ctx).await?;
        let tool_decision = collect_text(&mut *stream).await.unwrap_or_default();

        // Parse tool decision
        let tool_call: Option<serde_json::Value> = serde_json::from_str(&tool_decision).ok();

        if let Some(call) = tool_call {
            let tool_name = call["tool"].as_str().unwrap_or("unknown");
            let arguments = call["arguments"].to_string();
            let reasoning = call["reasoning"].as_str().unwrap_or("");

            println!("🔧 工具调用：{}", tool_name);
            println!("   参数：{}", arguments);
            println!("   理由：{}\n", reasoning);

            // Execute the tool
            let result = execute_tool(tool_name, &arguments)?;
            println!("📦 工具返回：{}\n", result);

            // Generate final response using tool result
            let mut final_builder = ModelContextBuilder::new();
            final_builder.prompt_text("system", "你是一个友好的助手，请根据工具返回的结果回答用户问题。");
            final_builder.user_text("user", *query);
            
            // Add tool call and result to context
            final_builder.add_tool_call_result(tool_name, &arguments, &result);
            
            // Add instruction for final response
            final_builder.user_text(
                "",
                &format!("工具 {} 返回了以下结果，请据此回答用户：\n{}", tool_name, result),
            );

            let final_ctx = final_builder.build();
            let mut final_stream = generator.generate_stream("", &final_ctx).await?;
            let response = collect_text(&mut *final_stream).await.unwrap_or_default();

            println!("💬 回答：{}\n", response.trim());
        } else {
            println!("⚠️ 无法解析工具调用，AI 直接回答：\n{}\n", tool_decision.trim());
        }
    }

    println!("============================================================");
    println!("  演示结束");
    println!("============================================================");

    Ok(())
}
