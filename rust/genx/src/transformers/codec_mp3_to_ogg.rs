use std::io::{Cursor, Read};
use std::sync::Arc;

use async_trait::async_trait;
use giztoy_audio::codec::{mp3, ogg, opus};
use tokio::sync::mpsc;

use crate::error::GenxError;
use crate::stream::{Stream, StreamResult};
use crate::transformer::Transformer;
use crate::types::MessageChunk;

type ConvertFn = dyn Fn(&[u8], &MP3ToOggConfig) -> Result<Vec<u8>, GenxError> + Send + Sync;

#[derive(Debug, Clone)]
pub struct MP3ToOggConfig {
    pub sample_rate: i32,
    pub channels: i32,
    pub bitrate: i32,
}

impl Default for MP3ToOggConfig {
    fn default() -> Self {
        Self {
            sample_rate: 48_000,
            channels: 1,
            bitrate: 64_000,
        }
    }
}

pub struct MP3ToOggTransformer {
    config: MP3ToOggConfig,
    converter: Arc<ConvertFn>,
}

impl MP3ToOggTransformer {
    pub fn new(config: MP3ToOggConfig) -> Self {
        Self {
            config,
            converter: Arc::new(convert_mp3_to_ogg),
        }
    }

    #[cfg(test)]
    fn with_converter(config: MP3ToOggConfig, converter: Arc<ConvertFn>) -> Self {
        Self { config, converter }
    }

    fn is_mp3_mime(mime: &str) -> bool {
        mime == "audio/mp3" || mime == "audio/mpeg"
    }

    async fn flush_mp3(
        &self,
        mp3_data: &mut Vec<u8>,
        last_chunk: Option<&MessageChunk>,
        tx: &mpsc::Sender<Result<MessageChunk, String>>,
    ) -> Result<(), GenxError> {
        if mp3_data.is_empty() {
            return Ok(());
        }

        let ogg_data = (self.converter)(mp3_data, &self.config)?;
        let mut out = MessageChunk::blob(crate::types::Role::User, "audio/ogg", ogg_data);
        if let Some(last) = last_chunk {
            out.role = last.role;
            out.name = last.name.clone();
        }

        tx.send(Ok(out))
            .await
            .map_err(|e| GenxError::Other(anyhow::anyhow!("send output chunk failed: {e}")))?;

        mp3_data.clear();
        Ok(())
    }
}

impl Default for MP3ToOggTransformer {
    fn default() -> Self {
        Self::new(MP3ToOggConfig::default())
    }
}

#[async_trait]
impl Transformer for MP3ToOggTransformer {
    async fn transform(
        &self,
        _pattern: &str,
        input: Box<dyn Stream>,
    ) -> Result<Box<dyn Stream>, GenxError> {
        let (tx, rx) = mpsc::channel(128);

        let config = self.config.clone();
        let converter = Arc::clone(&self.converter);

        tokio::spawn(async move {
            let this = MP3ToOggTransformer { config, converter };
            let mut input = input;
            let mut mp3_data = Vec::<u8>::new();
            let mut last_mp3_chunk: Option<MessageChunk> = None;

            loop {
                match input.next().await {
                    Ok(Some(chunk)) => {
                        if chunk.is_end_of_stream() {
                            if let Some(blob) = chunk.part.as_ref().and_then(|p| p.as_blob())
                                && MP3ToOggTransformer::is_mp3_mime(&blob.mime_type)
                            {
                                if let Err(e) =
                                    this.flush_mp3(&mut mp3_data, last_mp3_chunk.as_ref(), &tx).await
                                {
                                    let _ = tx.send(Err(e.to_string())).await;
                                    return;
                                }

                                let mut eos = MessageChunk::new_end_of_stream("audio/ogg");
                                if let Some(last) = last_mp3_chunk.as_ref() {
                                    eos.role = last.role;
                                    eos.name = last.name.clone();
                                } else {
                                    eos.role = chunk.role;
                                    eos.name = chunk.name.clone();
                                }
                                if tx.send(Ok(eos)).await.is_err() {
                                    return;
                                }
                                continue;
                            }

                            if tx.send(Ok(chunk)).await.is_err() {
                                return;
                            }
                            continue;
                        }

                        if let Some(blob) = chunk.part.as_ref().and_then(|p| p.as_blob())
                            && MP3ToOggTransformer::is_mp3_mime(&blob.mime_type)
                        {
                            mp3_data.extend_from_slice(&blob.data);
                            last_mp3_chunk = Some(chunk);
                            continue;
                        }

                        if tx.send(Ok(chunk)).await.is_err() {
                            return;
                        }
                    }
                    Ok(None) => {
                        if let Err(e) = this.flush_mp3(&mut mp3_data, last_mp3_chunk.as_ref(), &tx).await {
                            let _ = tx.send(Err(e.to_string())).await;
                        }
                        return;
                    }
                    Err(e) => {
                        let _ = tx.send(Err(e.to_string())).await;
                        return;
                    }
                }
            }
        });

        Ok(Box::new(ChannelStream { rx }))
    }
}

fn convert_mp3_to_ogg(mp3_data: &[u8], cfg: &MP3ToOggConfig) -> Result<Vec<u8>, GenxError> {
    if mp3_data.is_empty() {
        return Ok(Vec::new());
    }

    let mut dec = mp3::Mp3Decoder::new(Cursor::new(mp3_data));
    let mut test = [0u8; 4096];
    let _ = dec
        .read(&mut test)
        .map_err(|e| GenxError::Other(anyhow::anyhow!("detect mp3 format failed: {e}")))?;

    let mut sample_rate = dec.sample_rate();
    let mut channels = dec.channels();
    if sample_rate <= 0 {
        sample_rate = cfg.sample_rate;
    }
    if channels <= 0 {
        channels = cfg.channels;
    }

    if !matches!(sample_rate, 8_000 | 12_000 | 16_000 | 24_000 | 48_000) {
        sample_rate = cfg.sample_rate;
    }
    if !matches!(channels, 1 | 2) {
        channels = cfg.channels;
    }

    let mut dec = mp3::Mp3Decoder::new(Cursor::new(mp3_data));
    let mut enc = opus::Encoder::new_voip(sample_rate, channels)
        .map_err(|e| GenxError::Other(anyhow::anyhow!("create opus encoder failed: {e}")))?;
    enc.set_bitrate(cfg.bitrate)
        .map_err(|e| GenxError::Other(anyhow::anyhow!("set opus bitrate failed: {e}")))?;

    let mut ogg_buf = Vec::<u8>::new();
    let mut ogg_writer =
        ogg::OpusWriter::new(&mut ogg_buf, sample_rate as u32, channels as u16)
            .map_err(|e| GenxError::Other(anyhow::anyhow!("create ogg writer failed: {e}")))?;

    let frame_size = sample_rate * 20 / 1000;
    let bytes_per_frame = frame_size as usize * channels as usize * 2;
    let mut read_buf = vec![0u8; 4096];
    let mut pending = Vec::<u8>::with_capacity(bytes_per_frame * 2);

    loop {
        let n = dec
            .read(&mut read_buf)
            .map_err(|e| GenxError::Other(anyhow::anyhow!("read mp3 pcm failed: {e}")))?;

        if n == 0 {
            break;
        }
        pending.extend_from_slice(&read_buf[..n]);

        while pending.len() >= bytes_per_frame {
            let frame_pcm = pending[..bytes_per_frame].to_vec();
            let pcm_i16 = pcm_bytes_to_i16(&frame_pcm)?;
            let frame = enc
                .encode(&pcm_i16, frame_size)
                .map_err(|e| GenxError::Other(anyhow::anyhow!("opus encode failed: {e}")))?;
            ogg_writer
                .write_frame(frame.as_bytes(), i64::from(frame_size))
                .map_err(|e| GenxError::Other(anyhow::anyhow!("write ogg frame failed: {e}")))?;

            pending.drain(..bytes_per_frame);
        }
    }

    if !pending.is_empty() {
        let mut padded = vec![0u8; bytes_per_frame];
        padded[..pending.len()].copy_from_slice(&pending);
        let pcm_i16 = pcm_bytes_to_i16(&padded)?;
        let frame = enc
            .encode(&pcm_i16, frame_size)
            .map_err(|e| GenxError::Other(anyhow::anyhow!("opus encode tail frame failed: {e}")))?;
        ogg_writer
            .write_frame(frame.as_bytes(), i64::from(frame_size))
            .map_err(|e| GenxError::Other(anyhow::anyhow!("write ogg tail frame failed: {e}")))?;
    }

    ogg_writer
        .close()
        .map_err(|e| GenxError::Other(anyhow::anyhow!("close ogg writer failed: {e}")))?;

    Ok(ogg_buf)
}

fn pcm_bytes_to_i16(bytes: &[u8]) -> Result<Vec<i16>, GenxError> {
    if bytes.len() % 2 != 0 {
        return Err(GenxError::Other(anyhow::anyhow!(
            "pcm bytes length must be even, got {}",
            bytes.len()
        )));
    }
    let mut out = Vec::with_capacity(bytes.len() / 2);
    for chunk in bytes.chunks_exact(2) {
        out.push(i16::from_le_bytes([chunk[0], chunk[1]]));
    }
    Ok(out)
}

struct ChannelStream {
    rx: mpsc::Receiver<Result<MessageChunk, String>>,
}

#[async_trait]
impl Stream for ChannelStream {
    async fn next(&mut self) -> Result<Option<MessageChunk>, GenxError> {
        match self.rx.recv().await {
            Some(Ok(c)) => Ok(Some(c)),
            Some(Err(e)) => Err(GenxError::Other(anyhow::anyhow!("{e}"))),
            None => Ok(None),
        }
    }

    fn result(&self) -> Option<StreamResult> {
        None
    }

    async fn close(&mut self) -> Result<(), GenxError> {
        self.rx.close();
        Ok(())
    }

    async fn close_with_error(&mut self, _error: GenxError) -> Result<(), GenxError> {
        self.rx.close();
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::types::Role;

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

    fn fake_converter_ok() -> Arc<ConvertFn> {
        Arc::new(|mp3_data: &[u8], _cfg: &MP3ToOggConfig| {
            let mut out = b"OGG".to_vec();
            out.extend_from_slice(mp3_data);
            Ok(out)
        })
    }

    #[tokio::test]
    async fn t_codec_passthrough_non_mp3() {
        let t = MP3ToOggTransformer::with_converter(MP3ToOggConfig::default(), fake_converter_ok());
        let input = make_input(vec![
            MessageChunk::text(Role::User, "hello"),
            MessageChunk::blob(Role::User, "audio/wav", vec![1, 2, 3]),
            MessageChunk::blob(Role::User, "audio/ogg", vec![4, 5]),
        ]);

        let mut out = t.transform("", input).await.unwrap();
        let mut got = Vec::new();
        while let Some(c) = out.next().await.unwrap() {
            got.push(c);
        }

        assert_eq!(got.len(), 3);
        assert_eq!(got[0].part.as_ref().and_then(|p| p.as_text()), Some("hello"));
        assert_eq!(
            got[1]
                .part
                .as_ref()
                .and_then(|p| p.as_blob())
                .map(|b| b.mime_type.as_str()),
            Some("audio/wav")
        );
    }

    #[tokio::test]
    async fn t_codec_mp3_eos_handling() {
        let t = MP3ToOggTransformer::with_converter(MP3ToOggConfig::default(), fake_converter_ok());

        let mut eos = MessageChunk::new_end_of_stream("audio/mp3");
        eos.role = Role::Model;
        eos.name = Some("m1".into());

        let input = make_input(vec![
            MessageChunk::blob(Role::Model, "audio/mp3", vec![7, 8]),
            eos,
        ]);

        let mut out = t.transform("", input).await.unwrap();
        let c1 = out.next().await.unwrap().unwrap();
        let c2 = out.next().await.unwrap().unwrap();

        assert_eq!(
            c1.part
                .as_ref()
                .and_then(|p| p.as_blob())
                .map(|b| b.mime_type.as_str()),
            Some("audio/ogg")
        );
        assert!(c2.is_end_of_stream());
        assert_eq!(
            c2.part
                .as_ref()
                .and_then(|p| p.as_blob())
                .map(|b| b.mime_type.as_str()),
            Some("audio/ogg")
        );
        assert_eq!(c2.role, Role::Model);
        assert_eq!(c2.name.as_deref(), None);
    }

    #[tokio::test]
    async fn t_codec_passthrough_non_mp3_eos() {
        let t = MP3ToOggTransformer::with_converter(MP3ToOggConfig::default(), fake_converter_ok());
        let eos = MessageChunk::new_end_of_stream("audio/wav");
        let input = make_input(vec![eos.clone()]);
        let mut out = t.transform("", input).await.unwrap();
        let got = out.next().await.unwrap().unwrap();
        assert_eq!(got, eos);
    }

    #[tokio::test]
    async fn t_codec_input_error_propagation() {
        struct ErrStream;

        #[async_trait]
        impl Stream for ErrStream {
            async fn next(&mut self) -> Result<Option<MessageChunk>, GenxError> {
                Err(GenxError::Other(anyhow::anyhow!("boom")))
            }
            fn result(&self) -> Option<StreamResult> {
                None
            }
            async fn close(&mut self) -> Result<(), GenxError> {
                Ok(())
            }
            async fn close_with_error(&mut self, _error: GenxError) -> Result<(), GenxError> {
                Ok(())
            }
        }

        let t = MP3ToOggTransformer::with_converter(MP3ToOggConfig::default(), fake_converter_ok());
        let mut out = t.transform("", Box::new(ErrStream)).await.unwrap();
        let err = out.next().await.unwrap_err();
        assert!(err.to_string().contains("boom"));
    }

    #[tokio::test]
    async fn t_codec_eof_handling() {
        let t = MP3ToOggTransformer::with_converter(MP3ToOggConfig::default(), fake_converter_ok());
        let input = make_input(vec![MessageChunk::blob(Role::Model, "audio/mp3", vec![1, 2, 3])]);
        let mut out = t.transform("", input).await.unwrap();

        let chunk = out.next().await.unwrap().unwrap();
        assert_eq!(
            chunk
                .part
                .as_ref()
                .and_then(|p| p.as_blob())
                .map(|b| b.mime_type.as_str()),
            Some("audio/ogg")
        );
        assert!(out.next().await.unwrap().is_none());
    }

    #[tokio::test]
    async fn t_codec_mp3_eos_inherit_last_mp3_metadata() {
        let t = MP3ToOggTransformer::with_converter(MP3ToOggConfig::default(), fake_converter_ok());

        let mut last = MessageChunk::blob(Role::User, "audio/mp3", vec![9, 9]);
        last.name = Some("last-mp3".into());

        let mut eos = MessageChunk::new_end_of_stream("audio/mp3");
        eos.role = Role::Model;
        eos.name = Some("eos-meta".into());

        let input = make_input(vec![last, eos]);
        let mut out = t.transform("", input).await.unwrap();
        let _data = out.next().await.unwrap().unwrap();
        let eos_out = out.next().await.unwrap().unwrap();

        assert_eq!(eos_out.role, Role::User);
        assert_eq!(eos_out.name.as_deref(), Some("last-mp3"));
    }
}
