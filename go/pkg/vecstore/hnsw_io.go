package vecstore

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// Binary format version and magic bytes for HNSW serialization.
var hnswMagic = [4]byte{'H', 'N', 'S', 'W'}

const hnswVersion uint32 = 1

// Save serializes the entire HNSW index to w in a compact binary format.
//
// The format preserves internal node IDs so that neighbor references remain
// valid after deserialization. Deleted (free) slots are written as inactive
// markers to maintain index alignment.
//
// Format overview:
//
//	[4B magic "HNSW"] [4B version]
//	[4B dim] [4B M] [4B efConstruction] [4B efSearch]
//	[4B numSlots] [4B activeCount] [4B maxLevel] [4B entryID]
//	[4B freeCount] [freeCount × 4B free IDs]
//	For each slot:
//	  [1B active flag]
//	  If active:
//	    [4B idLen] [idLen bytes ID]
//	    [4B level]
//	    [dim × 4B float32 vector]
//	    For each layer 0..level:
//	      [4B numFriends] [numFriends × 4B friend IDs]
func (h *HNSW) Save(w io.Writer) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	bw := bufio.NewWriter(w)

	le := binary.LittleEndian
	write := func(v any) error { return binary.Write(bw, le, v) }

	// Header.
	if _, err := bw.Write(hnswMagic[:]); err != nil {
		return fmt.Errorf("vecstore: save magic: %w", err)
	}
	if err := write(hnswVersion); err != nil {
		return fmt.Errorf("vecstore: save version: %w", err)
	}

	// Config.
	for _, v := range []uint32{
		uint32(h.cfg.Dim),
		uint32(h.cfg.M),
		uint32(h.cfg.EfConstruction),
		uint32(h.cfg.EfSearch),
	} {
		if err := write(v); err != nil {
			return fmt.Errorf("vecstore: save config: %w", err)
		}
	}

	// Index metadata.
	if err := write(uint32(len(h.nodes))); err != nil {
		return err
	}
	if err := write(uint32(h.count)); err != nil {
		return err
	}
	if err := write(uint32(h.maxLevel)); err != nil {
		return err
	}
	if err := write(h.entryID); err != nil {
		return err
	}

	// Free list.
	if err := write(uint32(len(h.free))); err != nil {
		return err
	}
	for _, f := range h.free {
		if err := write(f); err != nil {
			return err
		}
	}

	// Nodes.
	for _, nd := range h.nodes {
		if nd == nil {
			if err := write(uint8(0)); err != nil {
				return err
			}
			continue
		}

		if err := write(uint8(1)); err != nil {
			return err
		}

		// External ID.
		idBytes := []byte(nd.id)
		if err := write(uint32(len(idBytes))); err != nil {
			return err
		}
		if _, err := bw.Write(idBytes); err != nil {
			return err
		}

		// Level.
		if err := write(uint32(nd.level)); err != nil {
			return err
		}

		// Vector.
		for _, v := range nd.vector {
			if err := write(v); err != nil {
				return err
			}
		}

		// Friend lists per layer.
		for lev := 0; lev <= nd.level; lev++ {
			var friends []uint32
			if lev < len(nd.friends) {
				friends = nd.friends[lev]
			}
			if err := write(uint32(len(friends))); err != nil {
				return err
			}
			for _, f := range friends {
				if err := write(f); err != nil {
					return err
				}
			}
		}
	}

	return bw.Flush()
}

// LoadHNSW deserializes an HNSW index from r. The returned index is ready
// for immediate use (Insert, Search, Delete).
func LoadHNSW(r io.Reader) (*HNSW, error) {
	br := bufio.NewReader(r)

	le := binary.LittleEndian
	read := func(v any) error { return binary.Read(br, le, v) }

	// Magic.
	var magic [4]byte
	if _, err := io.ReadFull(br, magic[:]); err != nil {
		return nil, fmt.Errorf("vecstore: load magic: %w", err)
	}
	if magic != hnswMagic {
		return nil, fmt.Errorf("vecstore: invalid magic %q", magic[:])
	}

	// Version.
	var version uint32
	if err := read(&version); err != nil {
		return nil, fmt.Errorf("vecstore: load version: %w", err)
	}
	if version != hnswVersion {
		return nil, fmt.Errorf("vecstore: unsupported version %d (want %d)", version, hnswVersion)
	}

	// Config.
	var dim, m, efC, efS uint32
	if err := read(&dim); err != nil {
		return nil, err
	}
	if dim == 0 {
		return nil, fmt.Errorf("vecstore: invalid dimension 0 in serialized index")
	}
	if err := read(&m); err != nil {
		return nil, err
	}
	if err := read(&efC); err != nil {
		return nil, err
	}
	if err := read(&efS); err != nil {
		return nil, err
	}

	// Metadata.
	var numSlots, activeCount, maxLev uint32
	var entryID int32
	if err := read(&numSlots); err != nil {
		return nil, err
	}
	if err := read(&activeCount); err != nil {
		return nil, err
	}
	if err := read(&maxLev); err != nil {
		return nil, err
	}
	if err := read(&entryID); err != nil {
		return nil, err
	}

	// Free list.
	var freeCount uint32
	if err := read(&freeCount); err != nil {
		return nil, err
	}
	free := make([]uint32, freeCount)
	for i := range free {
		if err := read(&free[i]); err != nil {
			return nil, err
		}
	}

	// Nodes.
	nodes := make([]*hnswNode, numSlots)
	idMap := make(map[string]uint32, activeCount)

	for i := uint32(0); i < numSlots; i++ {
		var active uint8
		if err := read(&active); err != nil {
			return nil, err
		}
		if active == 0 {
			continue
		}

		// External ID.
		var idLen uint32
		if err := read(&idLen); err != nil {
			return nil, err
		}
		idBytes := make([]byte, idLen)
		if _, err := io.ReadFull(br, idBytes); err != nil {
			return nil, err
		}

		// Level.
		var level uint32
		if err := read(&level); err != nil {
			return nil, err
		}

		// Vector.
		vec := make([]float32, dim)
		for j := range vec {
			if err := read(&vec[j]); err != nil {
				return nil, err
			}
		}

		// Friends.
		friends := make([][]uint32, level+1)
		for lev := uint32(0); lev <= level; lev++ {
			var nf uint32
			if err := read(&nf); err != nil {
				return nil, err
			}
			if nf > 0 {
				friends[lev] = make([]uint32, nf)
				for k := range friends[lev] {
					if err := read(&friends[lev][k]); err != nil {
						return nil, err
					}
				}
			}
		}

		nd := &hnswNode{
			id:      string(idBytes),
			vector:  vec,
			level:   int(level),
			friends: friends,
		}
		nodes[i] = nd
		idMap[nd.id] = i
	}

	cfg := HNSWConfig{
		Dim:            int(dim),
		M:              int(m),
		EfConstruction: int(efC),
		EfSearch:       int(efS),
	}
	cfg.setDefaults() // clamp M < 2 to avoid log(1)=0 → +Inf

	return &HNSW{
		cfg:      cfg,
		nodes:    nodes,
		idMap:    idMap,
		entryID:  entryID,
		maxLevel: int(maxLev),
		count:    int(activeCount),
		free:     free,
		levelMul: 1.0 / math.Log(float64(cfg.M)),
	}, nil
}
