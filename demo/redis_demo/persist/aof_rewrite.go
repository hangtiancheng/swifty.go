package persist

import (
	"io"
	"os"
	"time"

	"github.com/hangtiancheng/swifty.go/demo/redis_demo/database"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/datastore"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/handler"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/lib"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/log"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/protocol"
)

// rewriteAOF rewrites the AOF file.
func (a *aofPersister) rewriteAOF() error {
	// 1. Pre-rewrite: briefly acquire the lock.
	tmpFile, fileSize, err := a.startRewrite()
	if err != nil {
		return err
	}

	// 2. Rewrite AOF commands concurrently with the main loop.
	if err = a.doRewrite(tmpFile, fileSize); err != nil {
		return err
	}

	// 3. Post-rewrite: briefly acquire the lock.
	return a.endRewrite(tmpFile, fileSize)
}

func (a *aofPersister) startRewrite() (*os.File, int64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.aofFile.Sync(); err != nil {
		return nil, 0, err
	}

	fileInfo, _ := os.Stat(a.aofFileName)
	fileSize := fileInfo.Size()

	// Create a temporary AOF file.
	tmpFile, err := os.CreateTemp("./", "*.aof")
	if err != nil {
		return nil, 0, err
	}

	return tmpFile, fileSize, nil
}

func (a *aofPersister) doRewrite(tmpFile *os.File, fileSize int64) error {
	forkedDB, err := a.forkDB(fileSize)
	if err != nil {
		return err
	}

	// Convert db data to AOF commands.
	forkedDB.ForEach(func(key string, adapter database.CmdAdapter, expireAt *time.Time) {
		_, _ = tmpFile.Write(handler.NewMultiBulkReply(adapter.ToCmd()).ToBytes())

		if expireAt == nil {
			return
		}

		expireCmd := [][]byte{[]byte(database.CmdTypeExpireAt), []byte(key), []byte(lib.TimeSecondFormat(*expireAt))}
		_, _ = tmpFile.Write(handler.NewMultiBulkReply(expireCmd).ToBytes())
	})

	return nil
}

func (a *aofPersister) forkDB(fileSize int64) (database.DataStore, error) {
	file, err := os.Open(a.aofFileName)
	if err != nil {
		return nil, err
	}
	file.Seek(0, io.SeekStart)
	logger := log.GetDefaultLogger()
	reloader := readCloserAdapter(io.LimitReader(file, fileSize), file.Close)
	fakePersister := newFakePersister(reloader)
	tmpKVStore := datastore.NewKVStore(fakePersister)
	executor := database.NewDBExecutor(tmpKVStore)
	trigger := database.NewDBTrigger(executor)
	h, err := handler.NewHandler(trigger, fakePersister, protocol.NewParser(logger), logger)
	if err != nil {
		return nil, err
	}
	if err = h.Start(); err != nil {
		return nil, err
	}
	return tmpKVStore, nil
}

func (a *aofPersister) endRewrite(tmpFile *os.File, fileSize int64) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// copy commands executed during rewriting to tmpFile
	/* read write commands executed during rewriting */
	src, err := os.Open(a.aofFileName)
	if err != nil {
		return err
	}
	defer func() {
		_ = src.Close()
		_ = tmpFile.Close()
	}()

	if _, err = src.Seek(fileSize, 0); err != nil {
		return err
	}

	// Copy the tail of the old AOF file into the temp file.
	if _, err = io.Copy(tmpFile, src); err != nil {
		return err
	}

	// Close the old AOF file (about to be discarded).
	_ = a.aofFile.Close()
	// Rename the temp file to the new AOF file.
	if err := os.Rename(tmpFile.Name(), a.aofFileName); err != nil {
		// log
	}

	// Reopen the AOF file.
	aofFile, err := os.OpenFile(a.aofFileName, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		panic(err)
	}
	a.aofFile = aofFile
	return nil
}
