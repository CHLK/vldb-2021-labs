package commands

import (
	"encoding/hex"

	"github.com/pingcap-incubator/tinykv/kv/transaction/mvcc"
	"github.com/pingcap-incubator/tinykv/proto/pkg/kvrpcpb"
	"github.com/pingcap/log"
	"go.uber.org/zap"
)

type ResolveLock struct {
	CommandBase
	request  *kvrpcpb.ResolveLockRequest
	keyLocks []mvcc.KlPair
}

func NewResolveLock(request *kvrpcpb.ResolveLockRequest) ResolveLock {
	return ResolveLock{
		CommandBase: CommandBase{
			context: request.Context,
			startTs: request.StartVersion,
		},
		request: request,
	}
}

func (rl *ResolveLock) PrepareWrites(txn *mvcc.MvccTxn) (interface{}, error) {
	// A map from start timestamps to commit timestamps which tells us whether a transaction (identified by start ts)
	// has been committed (and if so, then its commit ts) or rolled back (in which case the commit ts is 0).
	commitTs := rl.request.CommitVersion
	response := new(kvrpcpb.ResolveLockResponse)

	log.Info("There keys to resolve",
		zap.Uint64("lockTS", txn.StartTS),
		zap.Int("number", len(rl.keyLocks)),
		zap.Uint64("commit_ts", commitTs))
	// panic("ResolveLock is not implemented yet")
	for _, kl := range rl.keyLocks {
		// YOUR CODE HERE (lab2).
		// Try to commit the key if the transaction is committed already, or try to rollback the key if it's not.
		// The `commitKey` and `rollbackKey` functions could be useful.
		log.Debug("resolve key", zap.String("key", hex.EncodeToString(kl.Key)))
		if kl.Lock.Kind == mvcc.WriteKindPut {
			pri := kl.Lock.Primary
			w, _, err := txn.CurrentWrite(pri)
			if err != nil {
				return response, err
			}
			if w == nil || w.Kind == mvcc.WriteKindRollback {
				kl.Lock.Kind = mvcc.WriteKindRollback
			}
		}
		switch kl.Lock.Kind {
		case mvcc.WriteKindRollback:
			responseRollback := new(kvrpcpb.BatchRollbackResponse)
			resp, e := rollbackKey(kl.Key, txn, responseRollback)
			if resp != nil {
				response.RegionError = responseRollback.RegionError
				response.Error = responseRollback.Error
			}
			if e != nil {
				return response, e
			}

		case mvcc.WriteKindPut:
			responseCommit := new(kvrpcpb.CommitResponse)
			resp, e := commitKey(kl.Key, commitTs, txn, responseCommit)
			if resp != nil {
				response.RegionError = responseCommit.RegionError
				response.Error = responseCommit.Error
			}
			if e != nil {
				return response, e
			}
		}
	}

	return response, nil
}

func (rl *ResolveLock) WillWrite() [][]byte {
	return nil
}

func (rl *ResolveLock) Read(txn *mvcc.RoTxn) (interface{}, [][]byte, error) {
	// Find all locks where the lock's transaction (start ts) is in txnStatus.
	txn.StartTS = rl.request.StartVersion
	keyLocks, err := mvcc.AllLocksForTxn(txn)
	if err != nil {
		return nil, nil, err
	}
	rl.keyLocks = keyLocks
	keys := [][]byte{}
	for _, kl := range keyLocks {
		keys = append(keys, kl.Key)
	}
	return nil, keys, nil
}
