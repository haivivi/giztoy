//! Stream utility functions: split, merge, composite.

use async_trait::async_trait;
use tokio::sync::mpsc;

use crate::error::{GenxError, Usage};
use crate::stream::{Stream, StreamResult};
use crate::types::MessageChunk;

/// A function that determines if a MessageChunk matches a criteria.
pub type Matcher = Box<dyn Fn(&MessageChunk) -> bool + Send + Sync>;

/// Returns a Matcher that matches chunks with the given MIME type prefix.
pub fn mime_type_matcher(prefix: String) -> Matcher {
    Box::new(move |chunk: &MessageChunk| {
        chunk
            .part
            .as_ref()
            .and_then(|p| p.as_blob())
            .is_some_and(|b| b.mime_type.starts_with(&prefix))
    })
}

/// Split a stream into two based on a matcher.
/// Chunks matching go to the first stream, others to the second.
pub fn split(
    mut input: Box<dyn Stream>,
    matcher: Matcher,
) -> (Box<dyn Stream>, Box<dyn Stream>) {
    let (matched_tx, matched_rx) = mpsc::channel(100);
    let (rest_tx, rest_rx) = mpsc::channel(100);

    tokio::spawn(async move {
        let mut matched_tx = Some(matched_tx);
        let mut rest_tx = Some(rest_tx);

        loop {
            if matched_tx.is_none() && rest_tx.is_none() {
                break;
            }
            match input.next().await {
                Ok(Some(chunk)) => {
                    if matcher(&chunk) {
                        if let Some(tx) = &matched_tx {
                            if tx.send(Ok(chunk)).await.is_err() {
                                matched_tx = None;
                            }
                        }
                    } else if let Some(tx) = &rest_tx {
                        if tx.send(Ok(chunk)).await.is_err() {
                            rest_tx = None;
                        }
                    }
                }
                Ok(None) => break,
                Err(e) => {
                    let msg = e.to_string();
                    if let Some(tx) = &matched_tx {
                        let _ = tx.send(Err(msg.clone())).await;
                    }
                    if let Some(tx) = &rest_tx {
                        let _ = tx.send(Err(msg)).await;
                    }
                    break;
                }
            }
        }
    });

    (
        Box::new(ChannelStream { rx: matched_rx }),
        Box::new(ChannelStream { rx: rest_rx }),
    )
}

/// Combine multiple streams sequentially.
/// After each stream ends (except the last), an EoS marker is emitted.
pub fn composite_seq(streams: Vec<Box<dyn Stream>>) -> Box<dyn Stream> {
    if streams.is_empty() {
        return Box::new(EmptyStream);
    }
    if streams.len() == 1 {
        return streams.into_iter().next().unwrap();
    }

    let (tx, rx) = mpsc::channel(100);

    tokio::spawn(async move {
        let count = streams.len();
        for (i, mut stream) in streams.into_iter().enumerate() {
            let mut last_mime = String::new();
            loop {
                match stream.next().await {
                    Ok(Some(chunk)) => {
                        if let Some(ref part) = chunk.part {
                            if let Some(blob) = part.as_blob() {
                                last_mime = blob.mime_type.clone();
                            } else if part.is_text() {
                                last_mime = "text/plain".into();
                            }
                        }
                        if tx.send(Ok(chunk)).await.is_err() {
                            return;
                        }
                    }
                    Ok(None) => break,
                    Err(e) => {
                        let _ = tx.send(Err(e.to_string())).await;
                        return;
                    }
                }
            }
            if i < count - 1 && !last_mime.is_empty() {
                let eos = if last_mime == "text/plain" {
                    MessageChunk::new_text_end_of_stream()
                } else {
                    MessageChunk::new_end_of_stream(&last_mime)
                };
                if tx.send(Ok(eos)).await.is_err() {
                    return;
                }
            }
        }
    });

    Box::new(ChannelStream { rx })
}

/// Merge multiple streams sequentially (all chunks from first, then second, etc.).
pub fn merge(streams: Vec<Box<dyn Stream>>) -> Box<dyn Stream> {
    if streams.is_empty() {
        return Box::new(EmptyStream);
    }
    if streams.len() == 1 {
        return streams.into_iter().next().unwrap();
    }
    Box::new(MergeStream {
        streams,
        idx: 0,
    })
}

/// Merge multiple streams by interleaving chunks round-robin.
pub fn merge_interleaved(streams: Vec<Box<dyn Stream>>) -> Box<dyn Stream> {
    if streams.is_empty() {
        return Box::new(EmptyStream);
    }
    if streams.len() == 1 {
        return streams.into_iter().next().unwrap();
    }

    let (tx, rx) = mpsc::channel(100);

    tokio::spawn(async move {
        let mut streams = streams;
        let mut active: Vec<bool> = vec![true; streams.len()];
        let mut active_count = streams.len();

        while active_count > 0 {
            for i in 0..streams.len() {
                if !active[i] {
                    continue;
                }
                match streams[i].next().await {
                    Ok(Some(chunk)) => {
                        if tx.send(Ok(chunk)).await.is_err() {
                            return;
                        }
                    }
                    Ok(None) => {
                        active[i] = false;
                        active_count -= 1;
                    }
                    Err(e) => {
                        let _ = tx.send(Err(e.to_string())).await;
                        return;
                    }
                }
            }
        }
    });

    Box::new(ChannelStream { rx })
}

struct ChannelStream {
    rx: mpsc::Receiver<Result<MessageChunk, String>>,
}

#[async_trait]
impl Stream for ChannelStream {
    async fn next(&mut self) -> Result<Option<MessageChunk>, GenxError> {
        match self.rx.recv().await {
            Some(Ok(chunk)) => Ok(Some(chunk)),
            Some(Err(e)) => Err(GenxError::Other(anyhow::anyhow!("{}", e))),
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

struct EmptyStream;

#[async_trait]
impl Stream for EmptyStream {
    async fn next(&mut self) -> Result<Option<MessageChunk>, GenxError> {
        Ok(None)
    }
    fn result(&self) -> Option<StreamResult> {
        Some(StreamResult::done(Usage::default()))
    }
    async fn close(&mut self) -> Result<(), GenxError> {
        Ok(())
    }
    async fn close_with_error(&mut self, _: GenxError) -> Result<(), GenxError> {
        Ok(())
    }
}

struct MergeStream {
    streams: Vec<Box<dyn Stream>>,
    idx: usize,
}

#[async_trait]
impl Stream for MergeStream {
    async fn next(&mut self) -> Result<Option<MessageChunk>, GenxError> {
        while self.idx < self.streams.len() {
            match self.streams[self.idx].next().await {
                Ok(Some(chunk)) => return Ok(Some(chunk)),
                Ok(None) => {
                    self.idx += 1;
                }
                Err(e) => return Err(e),
            }
        }
        Ok(None)
    }
    fn result(&self) -> Option<StreamResult> {
        None
    }
    async fn close(&mut self) -> Result<(), GenxError> {
        for s in &mut self.streams {
            let _ = s.close().await;
        }
        Ok(())
    }
    async fn close_with_error(&mut self, error: GenxError) -> Result<(), GenxError> {
        for s in &mut self.streams {
            let _ = s.close_with_error(GenxError::Other(anyhow::anyhow!("{}", error))).await;
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::stream::{collect_text, StreamBuilder};
    use crate::types::Role;

    fn make_text_stream(texts: &[&str]) -> Box<dyn Stream> {
        let builder = StreamBuilder::with_tools(64, vec![]);
        let chunks: Vec<MessageChunk> = texts
            .iter()
            .map(|t| MessageChunk::text(Role::Model, *t))
            .collect();
        builder.add(&chunks).unwrap();
        builder.done(Usage::default()).unwrap();
        Box::new(builder.stream())
    }

    fn make_mixed_stream() -> Box<dyn Stream> {
        let builder = StreamBuilder::with_tools(64, vec![]);
        builder
            .add(&[
                MessageChunk::text(Role::Model, "t1"),
                MessageChunk::blob(Role::Model, "audio/pcm", vec![1, 2]),
                MessageChunk::text(Role::Model, "t2"),
                MessageChunk::blob(Role::Model, "audio/pcm", vec![3, 4]),
                MessageChunk::text(Role::Model, "t3"),
            ])
            .unwrap();
        builder.done(Usage::default()).unwrap();
        Box::new(builder.stream())
    }

    #[tokio::test]
    async fn t8_1_split_by_mime() {
        let input = make_mixed_stream();
        let (mut matched, mut rest) = split(input, mime_type_matcher("audio/".into()));

        let mut matched_count = 0;
        while let Ok(Some(_)) = matched.next().await {
            matched_count += 1;
        }
        assert_eq!(matched_count, 2);

        let mut rest_count = 0;
        while let Ok(Some(_)) = rest.next().await {
            rest_count += 1;
        }
        assert_eq!(rest_count, 3);
    }

    #[tokio::test]
    async fn t8_2_split_all_match() {
        let builder = StreamBuilder::with_tools(64, vec![]);
        builder
            .add(&[
                MessageChunk::blob(Role::Model, "audio/pcm", vec![1]),
                MessageChunk::blob(Role::Model, "audio/mp3", vec![2]),
            ])
            .unwrap();
        builder.done(Usage::default()).unwrap();

        let (mut matched, mut rest) =
            split(Box::new(builder.stream()), mime_type_matcher("audio/".into()));

        let mut mc = 0;
        while let Ok(Some(_)) = matched.next().await {
            mc += 1;
        }
        assert_eq!(mc, 2);

        let r = rest.next().await.unwrap();
        assert!(r.is_none());
    }

    #[tokio::test]
    async fn t8_3_split_none_match() {
        let input = make_text_stream(&["a", "b"]);
        let (mut matched, mut rest) = split(input, mime_type_matcher("audio/".into()));

        let m = matched.next().await.unwrap();
        assert!(m.is_none());

        let mut rc = 0;
        while let Ok(Some(_)) = rest.next().await {
            rc += 1;
        }
        assert_eq!(rc, 2);
    }

    #[tokio::test]
    async fn t8_4_composite_seq_order() {
        let s1 = make_text_stream(&["a", "b"]);
        let s2 = make_text_stream(&["c"]);
        let s3 = make_text_stream(&["d", "e"]);

        let mut combined = composite_seq(vec![s1, s2, s3]);

        let mut texts = Vec::new();
        let mut eos_count = 0;
        while let Ok(Some(chunk)) = combined.next().await {
            if chunk.is_end_of_stream() {
                eos_count += 1;
            } else if let Some(t) = chunk.part.as_ref().and_then(|p| p.as_text()) {
                texts.push(t.to_string());
            }
        }
        assert_eq!(texts, vec!["a", "b", "c", "d", "e"]);
        assert_eq!(eos_count, 2); // EoS after s1 and s2, not after s3
    }

    #[tokio::test]
    async fn t8_5_composite_seq_empty_in_middle() {
        let s1 = make_text_stream(&["a"]);
        let s2 = make_text_stream(&[]); // empty stream
        let s3 = make_text_stream(&["b"]);

        let mut combined = composite_seq(vec![s1, s2, s3]);

        let mut texts = Vec::new();
        let mut eos_count = 0;
        while let Ok(Some(chunk)) = combined.next().await {
            if chunk.is_end_of_stream() {
                eos_count += 1;
            } else if let Some(t) = chunk.part.as_ref().and_then(|p| p.as_text()) {
                texts.push(t.to_string());
            }
        }
        assert_eq!(texts, vec!["a", "b"]);
        // EoS after s1 (has content), s2 is empty (no EoS emitted for empty),
        // but composite_seq emits EoS after every non-last stream that had content
        assert!(eos_count >= 1); // At least after s1
    }

    #[tokio::test]
    async fn t8_6_merge_sequential() {
        let s1 = make_text_stream(&["a", "b"]);
        let s2 = make_text_stream(&["c", "d"]);

        let mut merged = merge(vec![s1, s2]);
        let text = collect_text(&mut *merged).await.unwrap();
        assert_eq!(text, "abcd");
    }

    #[tokio::test]
    async fn t8_7_merge_first_ends() {
        let s1 = make_text_stream(&["x"]);
        let s2 = make_text_stream(&["y", "z"]);

        let mut merged = merge(vec![s1, s2]);
        let text = collect_text(&mut *merged).await.unwrap();
        assert_eq!(text, "xyz");
    }

    #[tokio::test]
    async fn t8_8_mime_type_matcher() {
        let matcher = mime_type_matcher("audio/".into());
        let audio_chunk = MessageChunk::blob(Role::Model, "audio/pcm", vec![1]);
        let text_chunk = MessageChunk::text(Role::Model, "hello");

        assert!(matcher(&audio_chunk));
        assert!(!matcher(&text_chunk));
    }
}
