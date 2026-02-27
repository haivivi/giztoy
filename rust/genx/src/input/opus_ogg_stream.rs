use std::io::{self, Read};

use anyhow::anyhow;
use async_trait::async_trait;
use giztoy_audio::codec::ogg::{OpusPacketIter, read_opus_packets};

use crate::error::{GenxError, Usage};
use crate::stream::{Stream, StreamResult};
use crate::types::{MessageChunk, Role};

use super::InputError;

/// 判断是否为 Opus 容器头包。
pub fn is_opus_header(data: &[u8]) -> bool {
    data.len() >= 8 && (data.starts_with(b"OpusHead") || data.starts_with(b"OpusTags"))
}

/// OGG Opus 输入流（按包流式产出，不做全量缓存）。
pub struct OpusOggStream<R: Read + Send + Sync> {
    packets: OpusPacketIter<R>,
    role: Role,
    name: String,
    seen_data_packet: bool,
    done: bool,
    closed: bool,
    close_error: Option<GenxError>,
}

impl<R: Read + Send + Sync> OpusOggStream<R> {
    pub fn new(reader: R, role: Role, name: impl Into<String>) -> Self {
        Self {
            packets: read_opus_packets(reader),
            role,
            name: name.into(),
            seen_data_packet: false,
            done: false,
            closed: false,
            close_error: None,
        }
    }

    fn ensure_open(&self) -> Result<(), GenxError> {
        if let Some(err) = &self.close_error {
            return Err(GenxError::Other(anyhow!(err.to_string())));
        }
        if self.closed {
            return Err(GenxError::Other(anyhow!(
                InputError::StreamClosed.to_string()
            )));
        }
        Ok(())
    }
}

#[async_trait]
impl<R: Read + Send + Sync> Stream for OpusOggStream<R> {
    async fn next(&mut self) -> Result<Option<MessageChunk>, GenxError> {
        self.ensure_open()?;
        if self.done {
            return Ok(None);
        }

        loop {
            match self.packets.next() {
                Some(Ok(packet)) => {
                    if is_opus_header(&packet.data) || packet.data.is_empty() {
                        continue;
                    }
                    self.seen_data_packet = true;
                    return Ok(Some(MessageChunk {
                        role: self.role,
                        name: if self.name.is_empty() {
                            None
                        } else {
                            Some(self.name.clone())
                        },
                        part: Some(crate::types::Part::blob("audio/opus", packet.data)),
                        tool_call: None,
                        ctrl: None,
                    }));
                }
                Some(Err(e)) => {
                    self.done = true;
                    return Err(GenxError::Other(anyhow!(
                        InputError::OggDecodeError(e.to_string()).to_string()
                    )));
                }
                None => {
                    self.done = true;
                    if !self.seen_data_packet {
                        return Err(GenxError::Other(anyhow!(
                            InputError::OggDecodeError("no opus packets found".into()).to_string()
                        )));
                    }
                    return Ok(None);
                }
            }
        }
    }

    fn result(&self) -> Option<StreamResult> {
        self.done.then(|| StreamResult::done(Usage::default()))
    }

    async fn close(&mut self) -> Result<(), GenxError> {
        self.closed = true;
        Ok(())
    }

    async fn close_with_error(&mut self, error: GenxError) -> Result<(), GenxError> {
        self.closed = true;
        self.close_error = Some(error);
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use giztoy_audio::codec::ogg::OpusWriter;

    fn create_valid_ogg(frames: &[&[u8]]) -> io::Result<Vec<u8>> {
        let mut out = Vec::new();
        {
            let mut writer = OpusWriter::new(&mut out, 48_000, 1)?;
            for f in frames {
                writer.write_frame(f, 960)?;
            }
            writer.close()?;
        }
        Ok(out)
    }

    #[test]
    fn t23_is_opus_header() {
        assert!(is_opus_header(b"OpusHead...."));
        assert!(is_opus_header(b"OpusTags...."));
        assert!(!is_opus_header(b"OpusHea"));
        assert!(!is_opus_header(b"random data"));
    }

    #[tokio::test]
    async fn t23_read_valid_ogg() {
        let data = create_valid_ogg(&[
            &[0x00, 0x01, 0x02],
            &[0x00, 0x03, 0x04],
            &[0x00, 0x05, 0x06],
        ])
        .unwrap();
        let mut stream = OpusOggStream::new(io::Cursor::new(data), Role::Model, "ogg");

        let mut n = 0;
        while let Some(chunk) = stream.next().await.unwrap() {
            let blob = chunk.part.as_ref().unwrap().as_blob().unwrap();
            assert_eq!(blob.mime_type, "audio/opus");
            n += 1;
        }
        assert_eq!(n, 3);
    }

    #[tokio::test]
    async fn t23_invalid_or_empty_data() {
        let mut invalid =
            OpusOggStream::new(io::Cursor::new(b"not valid ogg".to_vec()), Role::User, "x");
        assert!(invalid.next().await.is_err());

        let mut empty = OpusOggStream::new(io::Cursor::new(Vec::<u8>::new()), Role::User, "x");
        assert!(empty.next().await.is_err());
    }

    #[tokio::test]
    async fn t23_multiple_streams_and_large_file() {
        let part = create_valid_ogg(&[&[0x00, 0x01], &[0x00, 0x02]]).unwrap();
        let mut cat = part.clone();
        cat.extend_from_slice(&part);
        let mut stream = OpusOggStream::new(io::Cursor::new(cat), Role::User, "multi");
        let mut count = 0;
        while (stream.next().await.unwrap()).is_some() {
            count += 1;
        }
        assert!(count >= 2);

        let mut frames: Vec<Vec<u8>> = Vec::new();
        for i in 0..100u8 {
            frames.push(vec![0x00, i, i.wrapping_add(1)]);
        }
        let refs: Vec<&[u8]> = frames.iter().map(|v| v.as_slice()).collect();
        let data = create_valid_ogg(&refs).unwrap();
        let mut large = OpusOggStream::new(io::Cursor::new(data), Role::User, "large");
        let mut n = 0;
        while (large.next().await.unwrap()).is_some() {
            n += 1;
        }
        assert_eq!(n, 100);
    }
}
