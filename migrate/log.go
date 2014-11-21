package migrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"time"

	etcdserverpb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	etcd4pb "github.com/coreos/etcd/migrate/etcd4pb"
	"github.com/coreos/etcd/pkg/types"
	raftpb "github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/store"
)

const etcdDefaultClusterName = "etcd-cluster"

func UnixTimeOrPermanent(expireTime time.Time) int64 {
	expire := expireTime.Unix()
	if expireTime == store.Permanent {
		expire = 0
	}
	return expire
}

type Log4 []*etcd4pb.LogEntry

func (l Log4) NodeIDs() map[string]uint64 {
	out := make(map[string]uint64)
	for _, e := range l {
		if e.GetCommandName() == "etcd:join" {
			cmd4, err := NewCommand4(e.GetCommandName(), e.GetCommand(), nil)
			if err != nil {
				log.Println("error converting an etcd:join to v0.5 format. Likely corrupt!")
				return nil
			}
			join := cmd4.(*JoinCommand)
			m := generateNodeMember(join.Name, join.RaftURL, "")
			out[join.Name] = uint64(m.ID)
		}
		if e.GetCommandName() == "etcd:remove" {
			cmd4, err := NewCommand4(e.GetCommandName(), e.GetCommand(), nil)
			if err != nil {
				return nil
			}
			name := cmd4.(*RemoveCommand).Name
			delete(out, name)
		}
	}
	return out
}

func StorePath(key string) string {
	return path.Join("/1", key)
}

func DecodeLog4FromFile(logpath string) (Log4, error) {
	file, err := os.OpenFile(logpath, os.O_RDONLY, 0600)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return DecodeLog4(file)
}

func DecodeLog4(file *os.File) ([]*etcd4pb.LogEntry, error) {
	var readBytes int64
	entries := make([]*etcd4pb.LogEntry, 0)

	for {
		entry, n, err := DecodeNextEntry4(file)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed decoding next log entry: %v", err)
		}

		entries = append(entries, entry)

		readBytes += int64(n)
	}

	return entries, nil
}

// DecodeNextEntry4 unmarshals a v0.4 log entry from a reader. Returns the
// number of bytes read and any error that occurs.
func DecodeNextEntry4(r io.Reader) (*etcd4pb.LogEntry, int, error) {
	var length int
	_, err := fmt.Fscanf(r, "%8x\n", &length)
	if err != nil {
		return nil, -1, err
	}

	data := make([]byte, length)
	if _, err = io.ReadFull(r, data); err != nil {
		return nil, -1, err
	}

	ent4 := new(etcd4pb.LogEntry)
	if err = ent4.Unmarshal(data); err != nil {
		return nil, -1, err
	}

	// add width of scanner token to length
	length = length + 8 + 1

	return ent4, length, nil
}

func hashName(name string) uint64 {
	var sum uint64
	for _, ch := range name {
		sum = 131*sum + uint64(ch)
	}
	return sum
}

type Command4 interface {
	Type5() raftpb.EntryType
	Data5() ([]byte, error)
}

func NewCommand4(name string, data []byte, raftMap map[string]uint64) (Command4, error) {
	var cmd Command4

	switch name {
	case "etcd:remove":
		cmd = &RemoveCommand{}
	case "etcd:join":
		cmd = &JoinCommand{}
	case "etcd:setClusterConfig":
		cmd = &NOPCommand{}
	case "etcd:compareAndDelete":
		cmd = &CompareAndDeleteCommand{}
	case "etcd:compareAndSwap":
		cmd = &CompareAndSwapCommand{}
	case "etcd:create":
		cmd = &CreateCommand{}
	case "etcd:delete":
		cmd = &DeleteCommand{}
	case "etcd:set":
		cmd = &SetCommand{}
	case "etcd:sync":
		cmd = &SyncCommand{}
	case "etcd:update":
		cmd = &UpdateCommand{}
	case "raft:join":
		// These are subsumed by etcd:remove and etcd:join; we shouldn't see them.
		fallthrough
	case "raft:leave":
		return nil, fmt.Errorf("found a raft join/leave command; these shouldn't be in an etcd log")
	case "raft:nop":
		cmd = &NOPCommand{}
	default:
		return nil, fmt.Errorf("unregistered command type %s", name)
	}

	// If data for the command was passed in the decode it.
	if data != nil {
		if err := json.NewDecoder(bytes.NewReader(data)).Decode(cmd); err != nil {
			return nil, fmt.Errorf("unable to decode bytes %q: %v", data, err)
		}
	}

	switch name {
	case "etcd:join":
		c := cmd.(*JoinCommand)
		m := generateNodeMember(c.Name, c.RaftURL, c.EtcdURL)
		c.memb = *m
		if raftMap != nil {
			raftMap[c.Name] = uint64(m.ID)
		}
	case "etcd:remove":
		c := cmd.(*RemoveCommand)
		if raftMap != nil {
			m, ok := raftMap[c.Name]
			if !ok {
				return nil, fmt.Errorf("removing a node named %s before it joined", c.Name)
			}
			c.id = m
			delete(raftMap, c.Name)
		}
	}
	return cmd, nil
}

type RemoveCommand struct {
	Name string `json:"name"`
	id   uint64
}

func (c *RemoveCommand) Type5() raftpb.EntryType {
	return raftpb.EntryConfChange
}

func (c *RemoveCommand) Data5() ([]byte, error) {
	req5 := raftpb.ConfChange{
		ID:     0,
		Type:   raftpb.ConfChangeRemoveNode,
		NodeID: c.id,
	}
	return req5.Marshal()
}

type JoinCommand struct {
	Name    string `json:"name"`
	RaftURL string `json:"raftURL"`
	EtcdURL string `json:"etcdURL"`
	memb    member
}

func (c *JoinCommand) Type5() raftpb.EntryType {
	return raftpb.EntryConfChange
}

func (c *JoinCommand) Data5() ([]byte, error) {
	b, err := json.Marshal(c.memb)
	if err != nil {
		return nil, err
	}

	req5 := &raftpb.ConfChange{
		ID:      0,
		Type:    raftpb.ConfChangeAddNode,
		NodeID:  uint64(c.memb.ID),
		Context: b,
	}
	return req5.Marshal()
}

type SetClusterConfigCommand struct {
	Config *struct {
		ActiveSize   int     `json:"activeSize"`
		RemoveDelay  float64 `json:"removeDelay"`
		SyncInterval float64 `json:"syncInterval"`
	} `json:"config"`
}

func (c *SetClusterConfigCommand) Type5() raftpb.EntryType {
	return raftpb.EntryNormal
}

func (c *SetClusterConfigCommand) Data5() ([]byte, error) {
	b, err := json.Marshal(c.Config)
	if err != nil {
		return nil, err
	}

	req5 := &etcdserverpb.Request{
		Method: "PUT",
		Path:   "/v2/admin/config",
		Dir:    false,
		Val:    string(b),
	}

	return req5.Marshal()
}

type CompareAndDeleteCommand struct {
	Key       string `json:"key"`
	PrevValue string `json:"prevValue"`
	PrevIndex uint64 `json:"prevIndex"`
}

func (c *CompareAndDeleteCommand) Type5() raftpb.EntryType {
	return raftpb.EntryNormal
}

func (c *CompareAndDeleteCommand) Data5() ([]byte, error) {
	req5 := &etcdserverpb.Request{
		Method:    "DELETE",
		Path:      StorePath(c.Key),
		PrevValue: c.PrevValue,
		PrevIndex: c.PrevIndex,
	}
	return req5.Marshal()
}

type CompareAndSwapCommand struct {
	Key        string    `json:"key"`
	Value      string    `json:"value"`
	ExpireTime time.Time `json:"expireTime"`
	PrevValue  string    `json:"prevValue"`
	PrevIndex  uint64    `json:"prevIndex"`
}

func (c *CompareAndSwapCommand) Type5() raftpb.EntryType {
	return raftpb.EntryNormal
}

func (c *CompareAndSwapCommand) Data5() ([]byte, error) {
	req5 := &etcdserverpb.Request{
		Method:     "PUT",
		Path:       StorePath(c.Key),
		Val:        c.Value,
		PrevValue:  c.PrevValue,
		PrevIndex:  c.PrevIndex,
		Expiration: UnixTimeOrPermanent(c.ExpireTime),
	}
	return req5.Marshal()
}

type CreateCommand struct {
	Key        string    `json:"key"`
	Value      string    `json:"value"`
	ExpireTime time.Time `json:"expireTime"`
	Unique     bool      `json:"unique"`
	Dir        bool      `json:"dir"`
}

func (c *CreateCommand) Type5() raftpb.EntryType {
	return raftpb.EntryNormal
}

func (c *CreateCommand) Data5() ([]byte, error) {
	req5 := &etcdserverpb.Request{
		Path:       StorePath(c.Key),
		Dir:        c.Dir,
		Val:        c.Value,
		Expiration: UnixTimeOrPermanent(c.ExpireTime),
	}
	if c.Unique {
		req5.Method = "POST"
	} else {
		var prevExist = true
		req5.Method = "PUT"
		req5.PrevExist = &prevExist
	}
	return req5.Marshal()
}

type DeleteCommand struct {
	Key       string `json:"key"`
	Recursive bool   `json:"recursive"`
	Dir       bool   `json:"dir"`
}

func (c *DeleteCommand) Type5() raftpb.EntryType {
	return raftpb.EntryNormal
}

func (c *DeleteCommand) Data5() ([]byte, error) {
	req5 := &etcdserverpb.Request{
		Method:    "DELETE",
		Path:      StorePath(c.Key),
		Dir:       c.Dir,
		Recursive: c.Recursive,
	}
	return req5.Marshal()
}

type SetCommand struct {
	Key        string    `json:"key"`
	Value      string    `json:"value"`
	ExpireTime time.Time `json:"expireTime"`
	Dir        bool      `json:"dir"`
}

func (c *SetCommand) Type5() raftpb.EntryType {
	return raftpb.EntryNormal
}

func (c *SetCommand) Data5() ([]byte, error) {
	req5 := &etcdserverpb.Request{
		Method:     "PUT",
		Path:       StorePath(c.Key),
		Dir:        c.Dir,
		Val:        c.Value,
		Expiration: UnixTimeOrPermanent(c.ExpireTime),
	}
	return req5.Marshal()
}

type UpdateCommand struct {
	Key        string    `json:"key"`
	Value      string    `json:"value"`
	ExpireTime time.Time `json:"expireTime"`
}

func (c *UpdateCommand) Type5() raftpb.EntryType {
	return raftpb.EntryNormal
}

func (c *UpdateCommand) Data5() ([]byte, error) {
	exist := true
	req5 := &etcdserverpb.Request{
		Method:     "PUT",
		Path:       StorePath(c.Key),
		Val:        c.Value,
		PrevExist:  &exist,
		Expiration: UnixTimeOrPermanent(c.ExpireTime),
	}
	return req5.Marshal()
}

type SyncCommand struct {
	Time time.Time `json:"time"`
}

func (c *SyncCommand) Type5() raftpb.EntryType {
	return raftpb.EntryNormal
}

func (c *SyncCommand) Data5() ([]byte, error) {
	req5 := &etcdserverpb.Request{
		Method: "SYNC",
		Time:   c.Time.UnixNano(),
	}
	return req5.Marshal()
}

type DefaultJoinCommand struct {
	Name             string `json:"name"`
	ConnectionString string `json:"connectionString"`
}

type DefaultLeaveCommand struct {
	Name string `json:"name"`
	id   uint64
}

type NOPCommand struct{}

//TODO(bcwaldon): Why is CommandName here?
func (c NOPCommand) CommandName() string {
	return "raft:nop"
}

func (c *NOPCommand) Type5() raftpb.EntryType {
	return raftpb.EntryNormal
}

func (c *NOPCommand) Data5() ([]byte, error) {
	return nil, nil
}

func Entries4To5(ents4 []*etcd4pb.LogEntry) ([]raftpb.Entry, error) {
	ents4Len := len(ents4)

	if ents4Len == 0 {
		return nil, nil
	}

	startIndex := ents4[0].GetIndex()
	for i, e := range ents4[1:] {
		eIndex := e.GetIndex()
		// ensure indexes are monotonically increasing
		wantIndex := startIndex + uint64(i+1)
		if wantIndex != eIndex {
			return nil, fmt.Errorf("skipped log index %d", wantIndex)
		}
	}

	raftMap := make(map[string]uint64)
	ents5 := make([]raftpb.Entry, 0)
	for i, e := range ents4 {
		ent, err := toEntry5(e, raftMap)
		if err != nil {
			log.Fatalf("Error converting entry %d, %s", i, err)
		} else {
			ents5 = append(ents5, *ent)
		}
	}

	return ents5, nil
}

func toEntry5(ent4 *etcd4pb.LogEntry, raftMap map[string]uint64) (*raftpb.Entry, error) {
	cmd4, err := NewCommand4(ent4.GetCommandName(), ent4.GetCommand(), raftMap)
	if err != nil {
		return nil, err
	}

	data, err := cmd4.Data5()
	if err != nil {
		return nil, err
	}

	ent5 := raftpb.Entry{
		Term:  ent4.GetTerm(),
		Index: ent4.GetIndex(),
		Type:  cmd4.Type5(),
		Data:  data,
	}

	log.Printf("%d: %s -> %s", ent5.Index, ent4.GetCommandName(), ent5.Type)

	return &ent5, nil
}

func generateNodeMember(name, rafturl, etcdurl string) *member {
	pURLs, err := types.NewURLs([]string{rafturl})
	if err != nil {
		log.Fatalf("Invalid Raft URL %s -- this log could never have worked", rafturl)
	}

	m := NewMember(name, pURLs, etcdDefaultClusterName)
	m.ClientURLs = []string{etcdurl}
	return m
}
