package rootmulti

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	pruningtypes "cosmossdk.io/store/pruning/types"
	"cosmossdk.io/store/types"
)

// commitObservation is what a concurrent reader saw at one point: the latest
// CommitID and the value of the probe key in the store opened at that version.
type commitObservation struct {
	version int64
	hash    []byte
	value   []byte
}

// concurrentReaders runs n goroutines that read the store while Commit runs on
// the caller's goroutine, the way gRPC queries run while consensus commits
// blocks: like BaseApp.CreateQueryContext, each reader takes the latest
// version and opens the store at it with CacheMultiStoreWithVersion. Readers
// check that the version never goes backwards and record what they saw, so
// the caller can verify it against the real commits.
//
// (ABCI store queries, rootmulti.Store.Query, are serialized with Commit by
// the ABCI connection and are not exercised concurrently here.)
type concurrentReaders struct {
	wg           sync.WaitGroup
	stop         atomic.Bool
	mu           sync.Mutex
	observations []commitObservation
	errs         []error
}

func startConcurrentReaders(rs *Store, n int, key []byte) *concurrentReaders {
	r := &concurrentReaders{}
	for i := 0; i < n; i++ {
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			var lastVersion int64
			var seen []commitObservation
			// Check stop at the end, so every reader observes the store at least
			// once even when the writer finishes immediately.
			for done := false; !done; done = r.stop.Load() {
				cid := rs.LastCommitID()
				if cid.Version < lastVersion {
					r.fail(fmt.Errorf("LastCommitID went backwards: %d after %d", cid.Version, lastVersion))
					return
				}
				lastVersion = cid.Version

				if latest := rs.LatestVersion(); latest < cid.Version {
					r.fail(fmt.Errorf("LatestVersion %d is older than LastCommitID %d", latest, cid.Version))
					return
				}

				o := commitObservation{version: cid.Version, hash: cid.Hash}
				if cid.Version > 0 {
					cms, err := rs.CacheMultiStoreWithVersion(cid.Version)
					if err != nil {
						r.fail(fmt.Errorf("CacheMultiStoreWithVersion(%d): %w", cid.Version, err))
						return
					}
					o.value = cms.GetKVStore(testStoreKey1).Get(key)
				}
				seen = append(seen, o)
			}
			r.mu.Lock()
			r.observations = append(r.observations, seen...)
			r.mu.Unlock()
		}()
	}
	return r
}

func (r *concurrentReaders) fail(err error) {
	r.mu.Lock()
	r.errs = append(r.errs, err)
	r.mu.Unlock()
}

// committedVersion is what Commit produced for one version: its hash and the
// value of the probe key in store1.
type committedVersion struct {
	hash  []byte
	value []byte
}

// finish stops the readers and checks every observation against what Commit
// produced for that version.
func (r *concurrentReaders) finish(t testing.TB, committed map[int64]committedVersion) {
	t.Helper()
	r.stop.Store(true)
	r.wg.Wait()
	require.Empty(t, r.errs)
	require.NotEmpty(t, r.observations)
	for _, o := range r.observations {
		if o.version == 0 {
			continue
		}
		want, ok := committed[o.version]
		require.True(t, ok, "observed version %d was never committed", o.version)
		require.True(t, bytes.Equal(want.hash, o.hash), "version %d: observed hash %X, committed %X", o.version, o.hash, want.hash)
		require.True(t, bytes.Equal(want.value, o.value), "version %d: read %q, committed %q", o.version, o.value, want.value)
	}
}

// newConcurrencyTestStore returns a loaded store. disableFastNode mirrors the
// iavl-disable-fastnode setting: with the fast node enabled (IAVL's default),
// SaveVersion and reads share IAVL's fast storage version, which IAVL only
// synchronizes since v1.2.8.
func newConcurrencyTestStore(t testing.TB, disableFastNode bool) *Store {
	t.Helper()
	rs := newMultiStoreWithMounts(dbm.NewMemDB(), pruningtypes.NewPruningOptions(pruningtypes.PruningNothing))
	rs.SetIAVLDisableFastNode(disableFastNode)
	require.NoError(t, rs.LoadLatestVersion())
	return rs
}

// TestCommitConcurrentWithReads commits while readers query the latest
// version, as gRPC queries do during block execution. Run with -race: Commit
// used to write lastCommitInfo while LastCommitID read it unsynchronized.
func TestCommitConcurrentWithReads(t *testing.T) {
	for _, disableFastNode := range []bool{false, true} {
		t.Run(fmt.Sprintf("iavl-disable-fastnode=%v", disableFastNode), func(t *testing.T) {
			testCommitConcurrentWithReads(t, disableFastNode)
		})
	}
}

func testCommitConcurrentWithReads(t *testing.T, disableFastNode bool) {
	rs := newConcurrencyTestStore(t, disableFastNode)
	key := []byte("key")
	readers := startConcurrentReaders(rs, 8, key)

	committed := make(map[int64]committedVersion)
	for i := 0; i < 200; i++ {
		value := []byte(fmt.Sprintf("value-%d", i))
		rs.GetKVStore(testStoreKey1).Set(key, value)
		rs.GetKVStore(testStoreKey2).Set([]byte(fmt.Sprintf("k%d", i)), []byte{byte(i)})
		cid := rs.Commit()
		require.EqualValues(t, i+1, cid.Version)
		committed[cid.Version] = committedVersion{hash: cid.Hash, value: value}
	}

	readers.finish(t, committed)
	require.Equal(t, committed[200].hash, rs.LastCommitID().Hash)
}

// FuzzCommitConcurrentWithReads drives arbitrary sequences of writes, deletes
// and commits while readers query the latest commit concurrently, checking
// that every commit a reader observes is one Commit actually produced.
//
//	go test -race -run '^$' -fuzz FuzzCommitConcurrentWithReads ./rootmulti/
func FuzzCommitConcurrentWithReads(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3})
	f.Add([]byte{3, 3, 3, 0, 0, 3, 1, 3})
	f.Add(bytes.Repeat([]byte{0, 3}, 32))

	keys := []types.StoreKey{testStoreKey1, testStoreKey2, testStoreKey3}
	f.Fuzz(func(t *testing.T, ops []byte) {
		if len(ops) == 0 {
			return
		}
		if len(ops) > 256 {
			ops = ops[:256]
		}
		// The first byte picks the IAVL fast node setting.
		rs := newConcurrencyTestStore(t, ops[0]&1 == 1)
		ops = ops[1:]
		key := []byte("key")
		readers := startConcurrentReaders(rs, 4, key)

		committed := make(map[int64]committedVersion)
		var last types.CommitID
		var value []byte // current value of the probe key in store1
		commit := func() {
			last = rs.Commit()
			committed[last.Version] = committedVersion{hash: last.Hash, value: value}
		}
		for i, op := range ops {
			store := rs.GetKVStore(keys[int(op>>2)%len(keys)])
			k := []byte{0xff, op >> 4} // never collides with the probe key
			switch op % 4 {
			case 0:
				store.Set(k, []byte{op, byte(i)})
			case 1:
				value = []byte{op, byte(i), 1}
				rs.GetKVStore(testStoreKey1).Set(key, value)
			case 2:
				store.Delete(k)
			case 3:
				commit()
			}
		}
		commit()

		readers.finish(t, committed)
		require.Equal(t, last, rs.LastCommitID())
	})
}
