use std::path::Path;
use std::sync::{Arc, atomic::{AtomicUsize, Ordering}};

use async_trait::async_trait;
use giztoy_audio::codec::mp3;
use giztoy_genx::stream::{Stream, StreamResult};
use giztoy_genx::transformers::{
    MP3ToOggConfig, MP3ToOggTransformer, VoiceprintConfig, VoiceprintTransformer,
};
use giztoy_genx::{MessageChunk, Role, Transformer};
use giztoy_voiceprint::{Hasher, VoiceprintError, VoiceprintModel};
use tokio::sync::mpsc;

fn testdata(rel: &str) -> std::path::PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../testdata/genx/transformers")
        .join(rel)
}

struct ChannelStream {
    rx: mpsc::Receiver<Result<MessageChunk, String>>,
}

#[async_trait]
impl Stream for ChannelStream {
    async fn next(&mut self) -> Result<Option<MessageChunk>, giztoy_genx::GenxError> {
        match self.rx.recv().await {
            Some(Ok(c)) => Ok(Some(c)),
            Some(Err(e)) => Err(giztoy_genx::GenxError::Other(anyhow::anyhow!("{e}"))),
            None => Ok(None),
        }
    }
    fn result(&self) -> Option<StreamResult> {
        None
    }
    async fn close(&mut self) -> Result<(), giztoy_genx::GenxError> {
        self.rx.close();
        Ok(())
    }
    async fn close_with_error(&mut self, _error: giztoy_genx::GenxError) -> Result<(), giztoy_genx::GenxError> {
        self.rx.close();
        Ok(())
    }
}

fn make_input(chunks: Vec<MessageChunk>) -> Box<dyn Stream> {
    let (tx, rx) = mpsc::channel(64);
    tokio::spawn(async move {
        for chunk in chunks {
            if tx.send(Ok(chunk)).await.is_err() {
                break;
            }
        }
    });
    Box::new(ChannelStream { rx })
}

fn gen_mp3_bytes() -> Vec<u8> {
    let sample_rate = 16_000;
    let channels = 1;
    let samples = vec![0i16; (sample_rate / 10) as usize];
    let mut pcm = Vec::with_capacity(samples.len() * 2);
    for s in samples {
        pcm.extend_from_slice(&s.to_le_bytes());
    }

    let mut pcm_reader = std::io::Cursor::new(pcm);
    let mut out = Vec::new();
    let _ = mp3::encode_pcm_stream(&mut out, &mut pcm_reader, sample_rate, channels, None)
        .expect("generate mp3");
    out
}

#[tokio::test]
async fn e2e_codec_cross_lang_consistency() {
    let expected: serde_json::Value = serde_json::from_slice(
        &std::fs::read(testdata("go_compat_codec_expected.json")).expect("read codec golden"),
    )
    .expect("parse codec golden");

    let mut mp3_chunk = MessageChunk::blob(Role::User, "audio/mp3", gen_mp3_bytes());
    mp3_chunk.name = Some("source".into());

    let mut eos = MessageChunk::new_end_of_stream("audio/mp3");
    eos.role = Role::Model;
    eos.name = Some("eos-meta".into());

    let t = MP3ToOggTransformer::new(MP3ToOggConfig::default());
    let mut out = t.transform("", make_input(vec![mp3_chunk, eos])).await.unwrap();

    let _ogg = out.next().await.unwrap().unwrap();
    let eos_out = out.next().await.unwrap().unwrap();

    let got = serde_json::json!({
        "eos_mime": eos_out.part.as_ref().and_then(|p| p.as_blob()).map(|b| b.mime_type.clone()).unwrap_or_default(),
        "eos_role": eos_out.role.to_string(),
        "eos_name": eos_out.name.unwrap_or_default(),
    });

    assert_eq!(got, expected);
}

struct MockVoiceprintModel {
    dim: usize,
    seq: AtomicUsize,
}

impl VoiceprintModel for MockVoiceprintModel {
    fn extract(&self, _audio: &[u8]) -> Result<Vec<f32>, VoiceprintError> {
        let idx = self.seq.fetch_add(1, Ordering::Relaxed);
        let base = if idx < 2 { 1.0 } else { -1.0 };
        Ok(vec![base; self.dim])
    }
    fn dimension(&self) -> usize {
        self.dim
    }
}

#[tokio::test]
async fn e2e_voiceprint_cross_lang_consistency() {
    let expected: serde_json::Value = serde_json::from_slice(
        &std::fs::read(testdata("go_compat_voiceprint_expected.json"))
            .expect("read voiceprint golden"),
    )
    .expect("parse voiceprint golden");

    let dim = 8;
    let planes = vec![vec![1.0; dim]; 16];
    let hasher = Arc::new(Hasher::from_planes(dim, 16, planes));
    let model = Arc::new(MockVoiceprintModel {
        dim,
        seq: AtomicUsize::new(0),
    });

    let t = VoiceprintTransformer::new(
        model,
        hasher,
        VoiceprintConfig {
            segment_duration_ms: 200,
            sample_rate: 16_000,
            detector_window_size: 2,
            detector_min_ratio: 0.5,
        },
    );

    let mut eos = MessageChunk::new_end_of_stream("audio/pcm");
    eos.role = Role::Model;
    eos.name = Some("vp".into());

    let pcm = vec![0u8; 16_000 * 2 / 2];
    let mut out = t
        .transform(
            "",
            make_input(vec![
                MessageChunk::blob(Role::Model, "audio/pcm", pcm.clone()),
                MessageChunk::blob(Role::Model, "audio/pcm", pcm),
                eos,
            ]),
        )
        .await
        .unwrap();

    let _ = out.next().await.unwrap().unwrap();
    let _ = out.next().await.unwrap().unwrap();
    let eos_out = out.next().await.unwrap().unwrap();

    let got = serde_json::json!({
        "eos_role": eos_out.role.to_string(),
        "eos_name": eos_out.name.unwrap_or_default(),
        "eos_label_non_empty": eos_out.ctrl.as_ref().map(|c| !c.label.is_empty()).unwrap_or(false)
    });

    assert_eq!(got, expected);
}
