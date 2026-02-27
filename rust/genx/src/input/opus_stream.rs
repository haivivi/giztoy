use std::io;

use anyhow::anyhow;
use async_trait::async_trait;

use crate::error::{GenxError, Usage};
use crate::stream::{Stream, StreamResult};
use crate::types::{MessageChunk, Role};

use super::InputError;
use super::opus::OpusFrame;

/// 顺序 Opus 帧读取接口。
pub trait OpusReader: Send + Sync {
    fn read_frame(&mut self) -> io::Result<OpusFrame>;
}

/// 将顺序 OpusReader 包装为 genx Stream。
pub struct OpusStream<R: OpusReader> {
    reader: R,
    role: Role,
    name: String,
    closed: bool,
    close_error: Option<GenxError>,
    done: bool,
}

impl<R: OpusReader> OpusStream<R> {
    pub fn new(reader: R, role: Role, name: impl Into<String>) -> Self {
        Self {
            reader,
            role,
            name: name.into(),
            closed: false,
            close_error: None,
            done: false,
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
impl<R: OpusReader> Stream for OpusStream<R> {
    async fn next(&mut self) -> Result<Option<MessageChunk>, GenxError> {
        self.ensure_open()?;
        if self.done {
            return Ok(None);
        }

        match self.reader.read_frame() {
            Ok(frame) => Ok(Some(MessageChunk {
                role: self.role,
                name: if self.name.is_empty() {
                    None
                } else {
                    Some(self.name.clone())
                },
                part: Some(crate::types::Part::blob("audio/opus", frame.0)),
                tool_call: None,
                ctrl: None,
            })),
            Err(e) if e.kind() == io::ErrorKind::UnexpectedEof => {
                self.done = true;
                Ok(None)
            }
            Err(e) => Err(GenxError::Other(anyhow!(e))),
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
    use std::collections::VecDeque;

    struct MockOpusReader {
        frames: VecDeque<OpusFrame>,
        err: Option<io::Error>,
    }

    impl MockOpusReader {
        fn with_frames(frames: Vec<OpusFrame>) -> Self {
            Self {
                frames: VecDeque::from(frames),
                err: None,
            }
        }
    }

    impl OpusReader for MockOpusReader {
        fn read_frame(&mut self) -> io::Result<OpusFrame> {
            if let Some(f) = self.frames.pop_front() {
                return Ok(f);
            }
            if let Some(e) = self.err.take() {
                return Err(e);
            }
            Err(io::Error::new(io::ErrorKind::UnexpectedEof, "eof"))
        }
    }

    #[tokio::test]
    async fn t21_stream_reads_in_order() {
        let reader = MockOpusReader::with_frames(vec![
            OpusFrame(vec![0xf8, 0xff, 0xfe]),
            OpusFrame(vec![0xf8, 0x01, 0x02]),
            OpusFrame(vec![0xf8, 0x03, 0x04]),
        ]);
        let mut stream = OpusStream::new(reader, Role::User, "test");

        let mut n = 0;
        while let Some(chunk) = stream.next().await.unwrap() {
            n += 1;
            let blob = chunk.part.unwrap().as_blob().unwrap().clone();
            assert_eq!(blob.mime_type, "audio/opus");
            assert_eq!(chunk.role, Role::User);
        }
        assert_eq!(n, 3);
    }

    #[tokio::test]
    async fn t21_stream_close_and_close_with_error() {
        let reader = MockOpusReader::with_frames(vec![OpusFrame(vec![0xf8, 0xff, 0xfe])]);
        let mut stream = OpusStream::new(reader, Role::User, "c");
        stream.close().await.unwrap();
        assert!(stream.next().await.is_err());

        let reader = MockOpusReader::with_frames(vec![OpusFrame(vec![0xf8, 0xff, 0xfe])]);
        let mut stream = OpusStream::new(reader, Role::User, "c");
        stream
            .close_with_error(GenxError::Other(anyhow!("custom")))
            .await
            .unwrap();
        assert!(stream.next().await.is_err());
    }
}
