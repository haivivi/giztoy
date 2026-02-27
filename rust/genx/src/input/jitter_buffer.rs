use std::cmp::Ordering;
use std::collections::BinaryHeap;

/// 带可比较时间戳的数据包。
pub trait Timestamped<T> {
    fn timestamp(&self) -> T;
}

#[derive(Debug)]
struct HeapItem<T, P> {
    ts: T,
    seq: u64,
    packet: P,
}

impl<T: Ord, P> PartialEq for HeapItem<T, P> {
    fn eq(&self, other: &Self) -> bool {
        self.ts == other.ts && self.seq == other.seq
    }
}

impl<T: Ord, P> Eq for HeapItem<T, P> {}

impl<T: Ord, P> PartialOrd for HeapItem<T, P> {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}

impl<T: Ord, P> Ord for HeapItem<T, P> {
    fn cmp(&self, other: &Self) -> Ordering {
        // BinaryHeap 是大顶堆，这里反转比较实现“小顶堆”语义。
        // 同 timestamp 用 seq 保证稳定出队（先入先出）。
        other
            .ts
            .cmp(&self.ts)
            .then_with(|| other.seq.cmp(&self.seq))
    }
}

/// 按时间戳重排的抖动缓冲区。
pub struct JitterBuffer<T, P>
where
    T: Ord + Copy,
    P: Timestamped<T>,
{
    heap: BinaryHeap<HeapItem<T, P>>,
    max_items: usize,
    seq: u64,
}

impl<T, P> JitterBuffer<T, P>
where
    T: Ord + Copy,
    P: Timestamped<T>,
{
    pub fn new(max_items: usize) -> Self {
        Self {
            heap: BinaryHeap::new(),
            max_items,
            seq: 0,
        }
    }

    pub fn push(&mut self, packet: P) {
        if self.max_items == 0 {
            return;
        }
        let item = HeapItem {
            ts: packet.timestamp(),
            seq: self.seq,
            packet,
        };
        self.seq = self.seq.wrapping_add(1);
        self.heap.push(item);

        while self.heap.len() > self.max_items {
            let _ = self.heap.pop();
        }
    }

    pub fn pop(&mut self) -> Option<P> {
        self.heap.pop().map(|v| v.packet)
    }

    pub fn peek(&self) -> Option<&P> {
        self.heap.peek().map(|v| &v.packet)
    }

    pub fn len(&self) -> usize {
        self.heap.len()
    }

    pub fn is_empty(&self) -> bool {
        self.heap.is_empty()
    }

    pub fn clear(&mut self) {
        self.heap.clear();
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[derive(Clone, Debug, PartialEq, Eq)]
    struct TestPacket {
        ts: i64,
        data: &'static str,
    }

    impl Timestamped<i64> for TestPacket {
        fn timestamp(&self) -> i64 {
            self.ts
        }
    }

    #[test]
    fn t20_jb_basic_reorder() {
        let mut jb = JitterBuffer::<i64, TestPacket>::new(100);
        jb.push(TestPacket {
            ts: 300,
            data: "third",
        });
        jb.push(TestPacket {
            ts: 100,
            data: "first",
        });
        jb.push(TestPacket {
            ts: 200,
            data: "second",
        });

        assert_eq!(jb.pop().unwrap().data, "first");
        assert_eq!(jb.pop().unwrap().data, "second");
        assert_eq!(jb.pop().unwrap().data, "third");
        assert!(jb.pop().is_none());
    }

    #[test]
    fn t20_jb_max_items_drop_oldest() {
        let mut jb = JitterBuffer::<i64, TestPacket>::new(3);
        for i in 1..=5 {
            jb.push(TestPacket {
                ts: i * 100,
                data: "x",
            });
        }
        assert_eq!(jb.len(), 3);
        assert_eq!(jb.pop().unwrap().ts, 300);
        assert_eq!(jb.pop().unwrap().ts, 400);
        assert_eq!(jb.pop().unwrap().ts, 500);
    }

    #[test]
    fn t20_jb_duplicate_timestamp_keep_and_stable() {
        let mut jb = JitterBuffer::<i64, TestPacket>::new(10);
        jb.push(TestPacket { ts: 100, data: "a" });
        jb.push(TestPacket { ts: 100, data: "b" });
        jb.push(TestPacket { ts: 100, data: "c" });

        assert_eq!(jb.pop().unwrap().data, "a");
        assert_eq!(jb.pop().unwrap().data, "b");
        assert_eq!(jb.pop().unwrap().data, "c");
    }

    #[test]
    fn t20_jb_peek_and_clear() {
        let mut jb = JitterBuffer::<i64, TestPacket>::new(10);
        assert!(jb.peek().is_none());
        jb.push(TestPacket { ts: 200, data: "b" });
        jb.push(TestPacket { ts: 100, data: "a" });
        assert_eq!(jb.peek().unwrap().data, "a");
        assert_eq!(jb.len(), 2);
        jb.clear();
        assert!(jb.is_empty());
    }
}
