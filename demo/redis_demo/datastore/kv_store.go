// Copyright (c) 2026 hangtiancheng
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package datastore

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/hangtiancheng/swifty.go/demo/redis_demo/database"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/handler"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/lib"
)

type KVStore struct {
	data      map[string]any
	expiredAt map[string]time.Time

	expireTimeWheel SortedSet

	persister handler.Persister
}

func NewKVStore(persister handler.Persister) database.DataStore {
	return &KVStore{
		data:            make(map[string]any),
		expiredAt:       make(map[string]time.Time),
		expireTimeWheel: newSkiplist("expireTimeWheel"),
		persister:       persister,
	}
}

// expire
func (k *KVStore) Expire(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	ttl, err := strconv.ParseInt(string(args[1]), 10, 64)
	if err != nil {
		return handler.NewSyntaxErrReply()
	}
	if ttl <= 0 {
		return handler.NewErrReply("ERR invalid expire time")
	}

	expireAt := lib.TimeNow().Add(time.Duration(ttl) * time.Second)
	_cmd := [][]byte{[]byte(database.CmdTypeExpireAt), []byte(key), []byte(lib.TimeSecondFormat(expireAt))}
	return k.expireAt(cmd.Ctx(), _cmd, key, expireAt)
}

func (k *KVStore) ExpireAt(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	expiredAt, err := lib.ParseTimeSecondFormat(string((args[1])))
	if err != nil {
		return handler.NewSyntaxErrReply()
	}
	if expiredAt.Before(lib.TimeNow()) {
		return handler.NewErrReply("ERR invalid expire time")
	}

	return k.expireAt(cmd.Ctx(), cmd.Cmd(), key, expiredAt)
}

func (k *KVStore) expireAt(ctx context.Context, cmd [][]byte, key string, expireAt time.Time) handler.Reply {
	k.expire(key, expireAt)
	k.persister.PersistCmd(ctx, cmd) // persist
	return handler.NewOKReply()
}

// string
func (k *KVStore) Get(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	v, err := k.getAsString(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}
	if v == nil {
		return handler.NewNillReply()
	}
	return handler.NewBulkReply(v.Bytes())
}

func (k *KVStore) MGet(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	res := make([][]byte, 0, len(args))
	for _, arg := range args {
		v, err := k.getAsString(string(arg))
		if err != nil {
			return handler.NewErrReply(err.Error())
		}
		if v == nil {
			res = append(res, []byte("(nil)"))
			continue
		}
		res = append(res, v.Bytes())
	}

	return handler.NewMultiBulkReply(res)
}

func (k *KVStore) Set(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	value := string(args[1])

	// Support NX and EX flags.
	var (
		insertStrategy bool
		ttlStrategy    bool
		ttlSeconds     int64
		ttlIndex       = -1
	)

	for i := 2; i < len(args); i++ {
		flag := strings.ToLower(string(args[i]))
		switch flag {
		case "nx":
			insertStrategy = true
		case "ex":
			// Duplicate EX flag.
			if ttlStrategy {
				return handler.NewSyntaxErrReply()
			}
			if i == len(args)-1 {
				return handler.NewSyntaxErrReply()
			}
			ttl, err := strconv.ParseInt(string(args[i+1]), 10, 64)
			if err != nil {
				return handler.NewSyntaxErrReply()
			}
			if ttl <= 0 {
				return handler.NewErrReply("ERR invalid expire time")
			}

			ttlStrategy = true
			ttlSeconds = ttl
			ttlIndex = i
			i++
		default:
			return handler.NewSyntaxErrReply()
		}
	}

	// Strip the EX pair from args before persisting.
	if ttlIndex != -1 {
		args = append(args[:ttlIndex], args[ttlIndex+2:]...)
	}

	// Set the key.
	affected := k.put(key, value, insertStrategy)
	if affected > 0 && ttlStrategy {
		expireAt := lib.TimeNow().Add(time.Duration(ttlSeconds) * time.Second)
		_cmd := [][]byte{[]byte(database.CmdTypeExpireAt), []byte(key), []byte(lib.TimeSecondFormat(expireAt))}
		_ = k.expireAt(cmd.Ctx(), _cmd, key, expireAt) // this also persists the EX information
	}

	// Persist the SET command.
	if affected > 0 {
		k.persister.PersistCmd(cmd.Ctx(), append([][]byte{[]byte(database.CmdTypeSet)}, args...))
		return handler.NewIntReply(affected)
	}

	return handler.NewNillReply()
}

func (k *KVStore) MSet(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	if len(args)&1 == 1 {
		return handler.NewSyntaxErrReply()
	}

	for i := 0; i < len(args); i += 2 {
		_ = k.put(string(args[i]), string(args[i+1]), false)
	}

	k.persister.PersistCmd(cmd.Ctx(), cmd.Cmd())
	return handler.NewIntReply(int64(len(args) >> 1))
}

// list
func (k *KVStore) LPush(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	list, err := k.getAsList(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}

	if list == nil {
		list = newListEntity(key)
		k.putAsList(key, list)
	}

	for i := 1; i < len(args); i++ {
		list.LPush(args[i])
	}

	k.persister.PersistCmd(cmd.Ctx(), cmd.Cmd())
	return handler.NewIntReply(list.Len())
}

func (k *KVStore) LPop(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	var cnt int64
	if len(args) > 1 {
		rawCnt, err := strconv.ParseInt(string(args[1]), 10, 64)
		if err != nil {
			return handler.NewSyntaxErrReply()
		}
		if rawCnt < 1 {
			return handler.NewSyntaxErrReply()
		}
		cnt = rawCnt
	}

	list, err := k.getAsList(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}

	if list == nil {
		return handler.NewNillReply()
	}

	if cnt == 0 {
		cnt = 1
	}

	popped := list.LPop(cnt)
	if popped == nil {
		return handler.NewNillReply()
	}

	k.persister.PersistCmd(cmd.Ctx(), cmd.Cmd()) // persist

	if len(popped) == 1 {
		return handler.NewBulkReply(popped[0])
	}

	return handler.NewMultiBulkReply(popped)
}

func (k *KVStore) RPush(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	list, err := k.getAsList(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}

	if list == nil {
		list = newListEntity(key, args[1:]...)
		k.putAsList(key, list)
		return handler.NewIntReply(list.Len())
	}

	for i := 1; i < len(args); i++ {
		list.RPush(args[i])
	}

	k.persister.PersistCmd(cmd.Ctx(), cmd.Cmd()) // persist
	return handler.NewIntReply(list.Len())
}

func (k *KVStore) RPop(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	var cnt int64
	if len(args) > 1 {
		rawCnt, err := strconv.ParseInt(string(args[1]), 10, 64)
		if err != nil {
			return handler.NewSyntaxErrReply()
		}
		if rawCnt < 1 {
			return handler.NewSyntaxErrReply()
		}
		cnt = rawCnt
	}

	list, err := k.getAsList(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}

	if list == nil {
		return handler.NewNillReply()
	}

	if cnt == 0 {
		cnt = 1
	}

	popped := list.RPop(cnt)
	if popped == nil {
		return handler.NewNillReply()
	}

	k.persister.PersistCmd(cmd.Ctx(), cmd.Cmd()) // persist
	if len(popped) == 1 {
		return handler.NewBulkReply(popped[0])
	}

	return handler.NewMultiBulkReply(popped)
}

func (k *KVStore) LRange(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	if len(args) != 3 {
		return handler.NewSyntaxErrReply()
	}

	key := string(args[0])
	start, err := strconv.ParseInt(string(args[1]), 10, 64)
	if err != nil {
		return handler.NewSyntaxErrReply()
	}

	stop, err := strconv.ParseInt(string(args[2]), 10, 64)
	if err != nil {
		return handler.NewSyntaxErrReply()
	}

	list, err := k.getAsList(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}

	if list == nil {
		return handler.NewNillReply()
	}

	if got := list.Range(start, stop); got != nil {
		return handler.NewMultiBulkReply(got)
	}

	return handler.NewNillReply()
}

// set
func (k *KVStore) SAdd(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	set, err := k.getAsSet(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}

	if set == nil {
		set = newSetEntity(key)
		k.putAsSet(key, set)
	}

	var added int64
	for _, arg := range args[1:] {
		added += set.Add(string(arg))
	}

	k.persister.PersistCmd(cmd.Ctx(), cmd.Cmd()) // persist
	return handler.NewIntReply(added)
}

func (k *KVStore) SIsMember(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	if len(args) != 2 {
		return handler.NewSyntaxErrReply()
	}

	key := string(args[0])
	set, err := k.getAsSet(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}

	if set == nil {
		return handler.NewIntReply(0)
	}

	return handler.NewIntReply(set.Exist(string(args[1])))
}

func (k *KVStore) SRem(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	set, err := k.getAsSet(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}

	if set == nil {
		return handler.NewIntReply(0)
	}

	var remVal int64
	for _, arg := range args[1:] {
		remVal += set.Rem(string(arg))
	}

	if remVal > 0 {
		k.persister.PersistCmd(cmd.Ctx(), cmd.Cmd()) // persist
	}
	return handler.NewIntReply(remVal)
}

// hash
func (k *KVStore) HSet(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	if len(args)&1 != 1 {
		return handler.NewSyntaxErrReply()
	}

	key := string(args[0])
	hashmap, err := k.getAsHashMap(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}

	if hashmap == nil {
		hashmap = newHashMapEntity(key)
		k.putAsHashMap(key, hashmap)
	}

	for i := 0; i < len(args)-1; i += 2 {
		hKey := string(args[i+1])
		hVal := args[i+2]
		hashmap.Put(hKey, hVal)
	}

	k.persister.PersistCmd(cmd.Ctx(), cmd.Cmd()) // persist
	return handler.NewIntReply(int64((len(args) - 1) >> 1))
}

func (k *KVStore) HGet(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	hashmap, err := k.getAsHashMap(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}

	if hashmap == nil {
		return handler.NewNillReply()
	}

	if v := hashmap.Get(string(args[1])); v != nil {
		return handler.NewBulkReply(v)
	}

	return handler.NewNillReply()
}

func (k *KVStore) HDel(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	hashmap, err := k.getAsHashMap(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}

	if hashmap == nil {
		return handler.NewIntReply(0)
	}

	var remVal int64
	for _, arg := range args[1:] {
		remVal += hashmap.Del(string(arg))
	}

	if remVal > 0 {
		k.persister.PersistCmd(cmd.Ctx(), cmd.Cmd()) // persist
	}
	return handler.NewIntReply(remVal)
}

// sorted set
func (k *KVStore) ZAdd(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	if len(args)&1 != 1 {
		return handler.NewSyntaxErrReply()
	}

	key := string(args[0])
	var (
		scores  = make([]int64, 0, (len(args)-1)>>1)
		members = make([]string, 0, (len(args)-1)>>1)
	)

	for i := 0; i < len(args)-1; i += 2 {
		score, err := strconv.ParseInt(string(args[i+1]), 10, 64)
		if err != nil {
			return handler.NewSyntaxErrReply()
		}

		scores = append(scores, score)
		members = append(members, string(args[i+2]))
	}

	zset, err := k.getAsSortedSet(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}

	if zset == nil {
		zset = newSkiplist(key)
		k.putAsSortedSet(key, zset)
	}

	for i := 0; i < len(scores); i++ {
		zset.Add(scores[i], members[i])
	}

	k.persister.PersistCmd(cmd.Ctx(), cmd.Cmd()) // persist
	return handler.NewIntReply(int64(len(scores)))
}

func (k *KVStore) ZRangeByScore(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	if len(args) < 3 {
		return handler.NewSyntaxErrReply()
	}

	key := string(args[0])
	score1, err := strconv.ParseInt(string(args[1]), 10, 64)
	if err != nil {
		return handler.NewSyntaxErrReply()
	}
	score2, err := strconv.ParseInt(string(args[2]), 10, 64)
	if err != nil {
		return handler.NewSyntaxErrReply()
	}

	zset, err := k.getAsSortedSet(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}

	if zset == nil {
		return handler.NewNillReply()
	}

	rawRes := zset.Range(score1, score2)
	if len(rawRes) == 0 {
		return handler.NewNillReply()
	}

	res := make([][]byte, 0, len(rawRes))
	for _, item := range rawRes {
		res = append(res, []byte(item))
	}

	return handler.NewMultiBulkReply(res)
}

func (k *KVStore) ZRem(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	zset, err := k.getAsSortedSet(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}

	if zset == nil {
		return handler.NewIntReply(0)
	}

	var remVal int64
	for _, arg := range args {
		remVal += zset.Rem(string(arg))
	}

	if remVal > 0 {
		k.persister.PersistCmd(cmd.Ctx(), cmd.Cmd()) // persist
	}
	return handler.NewIntReply(remVal)
}
