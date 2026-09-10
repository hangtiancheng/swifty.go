package raft

// IsResponseMsg reports whether the message type is a response message.
func IsResponseMsg(typ MessageType) bool {
	return typ == MsgAppResp || typ == MsgHeartbeatResp || typ == MsgVoteResp || typ == MsgPreVoteResp
}

// numOfPendingConf returns the number of configuration change entries in ents.
func numOfPendingConf(ents []Entry) int {
	var n int
	for _, ent := range ents {
		if ent.Type == EntryConfChange {
			n++
		}
	}
	return n
}
