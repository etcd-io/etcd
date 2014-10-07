package migrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/coreos/etcd/etcdserver"
	etcdserverpb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	etcd4pb "github.com/coreos/etcd/migrate/etcd4pb"
	"github.com/coreos/etcd/pkg/types"
	raftpb "github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/store"
)

func DecodeLog4FromFile(logpath string) ([]*etcd4pb.LogEntry, error) {
	file, err := os.OpenFile(logpath, os.O_RDWR, 0600)
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
			return nil, fmt.Errorf("failed decoding next log entry: ", err)
		}

		if entry != nil {
			entries = append(entries, entry)
		}

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

func hashName(name string) int64 {
	var sum int64
	for _, ch := range name {
		sum = 131*sum + int64(ch)
	}
	return sum
}

type Command4 interface {
	Type5() raftpb.EntryType
	Data5() ([]byte, error)
}

func NewCommand4(name string, data []byte) (Command4, error) {
	var cmd Command4

	switch name {
	case "etcd:remove":
		cmd = &RemoveCommand{}
	case "etcd:join":
		cmd = &JoinCommand{}
	case "etcd:setClusterConfig":
		//TODO(bcwaldon): can this safely be discarded?
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
		cmd = &DefaultJoinCommand{}
	case "raft:leave":
		cmd = &DefaultLeaveCommand{}
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

	return cmd, nil
}

type RemoveCommand struct {
	Name string `json:"name"`
}

func (c *RemoveCommand) Type5() raftpb.EntryType {
	return raftpb.EntryNormal
}

func (c *RemoveCommand) Data5() ([]byte, error) {
	m := etcdserver.Member{
		ID: hashName(c.Name),
	}

	req5 := &etcdserverpb.Request{
		Method: "DELETE",
		Path:   m.StoreKey(),
	}

	return req5.Marshal()
}

type JoinCommand struct {
	Name    string `json:"name"`
	RaftURL string `json:"raftURL"`
	EtcdURL string `json:"etcdURL"`

	//TODO(bcwaldon): Should these be converted?
	//MinVersion int `json:"minVersion"`
	//MaxVersion int `json:"maxVersion"`
}

func (c *JoinCommand) Type5() raftpb.EntryType {
	return raftpb.EntryNormal
}

func (c *JoinCommand) Data5() ([]byte, error) {
	pURLs, err := types.NewURLs([]string{c.RaftURL})
	if err != nil {
		return nil, err
	}

	m := etcdserver.GenerateMember(c.Name, pURLs, nil)

	//TODO(bcwaldon): why doesn't this go through GenerateMember?
	m.ClientURLs = []string{c.EtcdURL}

	b, err := json.Marshal(*m)
	if err != nil {
		return nil, err
	}

	req5 := &etcdserverpb.Request{
		Method: "PUT",
		Path:   m.StoreKey(),
		Val:    string(b),

		// TODO(bcwaldon): Is this correct?
		Time: store.Permanent.Unix(),

		//TODO(bcwaldon): What is the new equivalent of Unique?
		//Unique: c.Unique,
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

		// TODO(bcwaldon): Is this correct?
		Time: store.Permanent.Unix(),
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
		Path:      c.Key,
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
		Method:    "PUT",
		Path:      c.Key,
		Val:       c.Value,
		PrevValue: c.PrevValue,
		PrevIndex: c.PrevIndex,
		Time:      c.ExpireTime.Unix(),
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
		Method: "PUT",
		Path:   c.Key,
		Dir:    c.Dir,
		Val:    c.Value,

		// TODO(bcwaldon): Is this correct?
		Time: c.ExpireTime.Unix(),

		//TODO(bcwaldon): What is the new equivalent of Unique?
		//Unique: c.Unique,
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
		Path:      c.Key,
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
		Method: "PUT",
		Path:   c.Key,
		Dir:    c.Dir,
		Val:    c.Value,

		//TODO(bcwaldon): Is this correct?
		Time: c.ExpireTime.Unix(),
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
	req5 := &etcdserverpb.Request{
		Method: "PUT",
		Path:   c.Key,
		Val:    c.Value,

		//TODO(bcwaldon): Is this correct?
		Time: c.ExpireTime.Unix(),
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
		//TODO(bcwaldon): Is this correct?
		Time: c.Time.UnixNano(),
	}
	return req5.Marshal()
}

type DefaultJoinCommand struct {
	//TODO(bcwaldon): implement Type5, Data5
	Command4

	Name             string `json:"name"`
	ConnectionString string `json:"connectionString"`
}

type DefaultLeaveCommand struct {
	//TODO(bcwaldon): implement Type5, Data5
	Command4

	Name string `json:"name"`
}

//TODO(bcwaldon): Why is CommandName here?
func (c *DefaultLeaveCommand) CommandName() string {
	return "raft:leave"
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

func Entries4To5(commitIndex uint64, ents4 []*etcd4pb.LogEntry) ([]raftpb.Entry, error) {
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

	ents5 := make([]raftpb.Entry, 0)
	for i, e := range ents4 {
		ent, err := toEntry5(e)
		if err != nil {
			log.Printf("Ignoring invalid log data in entry %d: %v", i, err)
		} else {
			ents5 = append(ents5, *ent)
		}
	}

	return ents5, nil
}

func toEntry5(ent4 *etcd4pb.LogEntry) (*raftpb.Entry, error) {
	cmd4, err := NewCommand4(ent4.GetCommandName(), ent4.GetCommand())
	if err != nil {
		return nil, err
	}

	data, err := cmd4.Data5()
	if err != nil {
		return nil, err
	}

	ent5 := raftpb.Entry{
		Term:  int64(ent4.GetTerm()),
		Index: int64(ent4.GetIndex()),
		Type:  cmd4.Type5(),
		Data:  data,
	}

	log.Printf("%d: %s -> %s", ent5.Index, ent4.GetCommandName(), ent5.Type)

	return &ent5, nil
}
