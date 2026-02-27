use std::collections::VecDeque;
use std::time::Duration;

use async_trait::async_trait;
use giztoy_genx::transformers::{
    DashScopeRealtime, DashScopeSdkConnector, DoubaoRealtime, DoubaoSdkConnector,
};
use giztoy_genx::{MessageChunk, Role, Stream, StreamResult, Transformer};

struct VecInputStream {
    chunks: VecDeque<MessageChunk>,
}

#[async_trait]
impl Stream for VecInputStream {
    async fn next(&mut self) -> Result<Option<MessageChunk>, giztoy_genx::GenxError> {
        Ok(self.chunks.pop_front())
    }

    fn result(&self) -> Option<StreamResult> {
        None
    }

    async fn close(&mut self) -> Result<(), giztoy_genx::GenxError> {
        Ok(())
    }

    async fn close_with_error(&mut self, _error: giztoy_genx::GenxError) -> Result<(), giztoy_genx::GenxError> {
        Ok(())
    }
}

#[tokio::test]
#[ignore]
async fn test_doubao_realtime_contract() {
    let app_id = std::env::var("DOUBAO_APP_ID").expect("DOUBAO_APP_ID required");
    let token = std::env::var("DOUBAO_TOKEN").expect("DOUBAO_TOKEN required");

    let client = giztoy_doubaospeech::Client::builder(app_id)
        .bearer_token(token)
        .build()
        .expect("build doubao client");

    let cfg = giztoy_doubaospeech::RealtimeConfig::default();
    let transformer = DoubaoRealtime::new(std::sync::Arc::new(DoubaoSdkConnector::new(client, cfg)));

    let input = VecInputStream {
        chunks: VecDeque::from(vec![
            MessageChunk::new_begin_of_stream("contract-stream"),
            MessageChunk::blob(Role::User, "audio/pcm", vec![0; 3200]),
            MessageChunk::new_end_of_stream("audio/pcm"),
        ]),
    };

    let mut output = transformer
        .transform("doubao/realtime", Box::new(input))
        .await
        .expect("transform start");

    let _ = tokio::time::timeout(Duration::from_secs(20), output.next())
        .await
        .expect("timeout waiting first event")
        .expect("stream error");
}

#[tokio::test]
#[ignore]
async fn test_dashscope_realtime_contract() {
    let api_key = std::env::var("DASHSCOPE_API_KEY").expect("DASHSCOPE_API_KEY required");

    let client = giztoy_dashscope::Client::new(api_key).expect("build dashscope client");
    let cfg = giztoy_dashscope::RealtimeConfig {
        model: giztoy_dashscope::MODEL_QWEN_OMNI_TURBO_REALTIME_LATEST.to_string(),
    };
    let transformer = DashScopeRealtime::new(std::sync::Arc::new(DashScopeSdkConnector::new(client, cfg)));

    let input = VecInputStream {
        chunks: VecDeque::from(vec![
            MessageChunk::new_begin_of_stream("contract-stream"),
            MessageChunk::blob(Role::User, "audio/pcm", vec![0; 3200]),
            MessageChunk::new_end_of_stream("audio/pcm"),
        ]),
    };

    let mut output = transformer
        .transform("dashscope/realtime", Box::new(input))
        .await
        .expect("transform start");

    let _ = tokio::time::timeout(Duration::from_secs(20), output.next())
        .await
        .expect("timeout waiting first event")
        .expect("stream error");
}
