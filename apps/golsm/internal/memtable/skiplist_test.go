package memtable

import (
	"bytes"
	"reflect"
	"testing"
)

func Test_Skiplist(t *testing.T) {
	skiplist := NewSkiplist()
	skiplist.Put([]byte("a"), []byte("b"))
	skiplist.Put([]byte("a"), []byte("c"))
	skiplist.Put([]byte("ab"), []byte("aa"))
	skiplist.Put([]byte("abc"), []byte("aaa"))
	skiplist.Put([]byte("bc"), []byte("bbb"))
	skiplist.Put([]byte("ab"), []byte("bb"))

	expectKVs := []*KV{
		{Key: []byte("a"), Value: []byte("c")},
		{Key: []byte("ab"), Value: []byte("bb")},
		{Key: []byte("abc"), Value: []byte("aaa")},
		{Key: []byte("bc"), Value: []byte("bbb")},
	}

	for _, kv := range expectKVs {
		val, ok := skiplist.Get(kv.Key)
		if !ok {
			t.Errorf("key: %s, expect exist: true, got: false", kv.Key)
			continue
		}
		if !bytes.Equal(val, kv.Value) {
			t.Errorf("key: %s, expect value: %s, got: %s", kv.Key, kv.Value, val)
		}
	}

	if val, ok := skiplist.Get([]byte("bcd")); ok {
		t.Errorf("key: bcd, expect exist: false, got: true, value: %s", val)
	}

	// 4 distinct keys holding "c"(1) + "bb"(2) + "aaa"(3) + "bbb"(3) bytes of
	// values plus "a"+"ab"+"abc"+"bc" bytes of keys: 9 + 8 = 17 bytes.
	if got := skiplist.Size(); got != 17 {
		t.Errorf("expect size: 17, got: %d", got)
	}

	if got := skiplist.EntriesCnt(); got != 4 {
		t.Errorf("expect entries count: 4, got: %d", got)
	}

	kvs := skiplist.All()
	if !reflect.DeepEqual(kvs, expectKVs) {
		t.Errorf("expect kvs: %v, got: %v", expectKVs, kvs)
	}
}
