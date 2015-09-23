

package main

import (
    "strconv"
    "strings"
    "os"
    "fmt"  
    "bytes"
    "flag"
    "time"
    "sync"
    "log"   
     
    "crypto/rand"
    "os/exec"
    "math/big"
    "io/ioutil"

    "code.google.com/p/gcfg"
    "code.google.com/p/go.crypto/ssh"
    "github.com/coreos/go-etcd/etcd"
 
)

/*
Declarations :::: 
    actions     : for passing other functions as arguments to handler function.
    operation   : which operation to perform.
    keycount    : number of keys to be added, retrieved, deleted or updated.
    threads     : number of total threads.
    pct         : each entry represents percentage of values lying in .
                  (value_range[i],value_range[i+1]) . 
                  Example -- pct={10,10,50,30}, for value_range={0,16,32,64,128} .
    pct_count   : distribution of keys according to pct .
                  Example -- {100,100,500,300} for keycount=1000 .
    value_range : value range for keys .
    results     : result struct from report.go .
    threads     : Maximum number of threads allowed to run.
    threads_dist: Distribution of threads according to load
*/

type actions func(int,int)


var (
    wg sync.WaitGroup
    operation, conf_file string
    etcdmem_s, etcdmem_e int
    pct, pct_count, value_range, thread_dist []int
    client *etcd.Client
    start time.Time
    f *os.File
    err error
    key ssh.Signer
    config *ssh.ClientConfig
    ssh_client *ssh.Client
    session *ssh.Session
    mem_flag, remote_flag bool
    results chan *result
)

// Config variables
var (
    log_file, etcdhost, etcdport, remote_host, ssh_port,remote_host_user string
    keycount, operation_count, threads int
)

// Flag variables are descirbed here .
var (
    fhost, fport, foperation, flog_file, fcfg_file *string
    fkeycount, foperation_count *int
    fremote_flag, fmem_flag, fhelp *bool
)

 func init() {
    // All the defaults mentioned here refer to etcd_load.cfg values.
    fhelp = flag.Bool("help", false, "shows how to use flags")    
    fhost = flag.String("h", "null", "etcd instance address."+
                        "Default=127.0.0.1 from config file")
    fport = flag.String("p", "null", 
                        "port on which etcd is running. Defalt=4001")
    foperation = flag.String("o", "null", 
                        "operation - create/delete/get/update. Default:create")
    fkeycount = flag.Int("k",-1,"number of keys involved in operation,"+
                        "useful for create. Default:100")
    foperation_count = flag.Int("oc",-1,"number of operations to be performed,"+
                        " Default:200")
    flog_file = flag.String("log","null", "logfile name, default : log")
    fremote_flag = flag.Bool("remote",false," Must be set true if etcd "+
                        "instance is remote. Default=false")
    fmem_flag = flag.Bool("mem",false,"When true, memory info is shown."+
                        " Default=false")
    fcfg_file = flag.String("c","null","Input the cfg file. Required")
 }


func main() {
    flag.Parse()

    if *fhelp {
        flag.Usage()
        return
    }
    if *fcfg_file != "null"{
        conf_file = *fcfg_file
    } else
    {
        flag.Usage()
        fmt.Println("Please input the cfg file")
        return
    }
    
    readConfig()

    handleFlags()
    
    // Fetch the memory information before load test
    if mem_flag {
        memHandler(false)
    }

    // Creating a new client for handling requests .
    var machines = []string{"http://"+etcdhost+":"+etcdport}
    client =  etcd.NewClient(machines)

    // Log file is opened or created for storing log entries.
    f, err = os.OpenFile(log_file, os.O_RDWR | os.O_CREATE , 0666)
    if err != nil {
        log.Fatalf("error opening file: %v", err)
    }
    // Log file set
    log.SetOutput(f)
    log.Println("Starting #####")
    log.Println("Keycount, operation_count =",keycount,operation_count)


    // n result channels are made to pass time information during execution.
    // This part is useful for generating the commandline report.
    n := operation_count
    results = make(chan *result, n)
    
    // This is necessary for the goroutines.
    wg.Add(len(pct))
    
    // This part is where requests are handled.
    start := time.Now()
    switch{
    case operation == "create":
        log.Println("Operation : create")
        var values [2]int
        base := 0
        for i:=0;i<len(pct);i++{
            values[0] = value_range[i]
            values[1] = value_range[i+1]
            go create_keys(base,pct_count[i],values)
            base = base + pct_count[i]
        }
        wg.Wait()
        printReport(n, results, time.Now().Sub(start))
    case operation == "get":
        log.Println("Operation : get")
        handler(get_values)
        wg.Wait()
        printReport(n, results, time.Now().Sub(start))
    case operation == "update":
        log.Println("Operation : update")
        handler(update_values)
        wg.Wait()
        printReport(n, results, time.Now().Sub(start))
    case operation == "delete":
        log.Println("Operation : delete")
        handler(delete_values)
        wg.Wait()
        printReport(n, results, time.Now().Sub(start))
    }

    // Fetch and print memory information after load test
    if mem_flag{
        memHandler(true)
    }
    
    defer f.Close()
}


func readConfig(){
    //This struct is used to capture the configuration from the config file.
    cfg := struct {
        Section_Args struct {
            Etcdhost string
            Etcdport string
            Operation string
            Keycount string
            Operation_Count string
            Log_File string
            Threads int
            Pct string
            Value_Range string
            Remote_Flag bool
            Ssh_Port string
            Remote_Host_User string
        }
    }{}

    err = gcfg.ReadFileInto(&cfg, conf_file)
    if err != nil {
        log.Fatalf("Failed to parse gcfg data: ", err)
    }


    // Reading parameters from the config file and doing processing if needed.
    etcdhost = cfg.Section_Args.Etcdhost
    etcdport = cfg.Section_Args.Etcdport
    operation = cfg.Section_Args.Operation
    keycount = int(toInt(cfg.Section_Args.Keycount,10,64))
    operation_count = int(toInt(cfg.Section_Args.Operation_Count,10,64))
    log_file = cfg.Section_Args.Log_File
    remote_flag = cfg.Section_Args.Remote_Flag
    remote_host = cfg.Section_Args.Etcdhost
    ssh_port = cfg.Section_Args.Ssh_Port
    remote_host_user = cfg.Section_Args.Remote_Host_User
    threads = cfg.Section_Args.Threads

    // Calculate pct_count based on keycount and pct.
    percents := cfg.Section_Args.Pct
    temp := strings.Split(percents,",")
    pct = make([]int,len(temp))
    pct_count = make([]int,len(temp))
    for i:=0;i<len(temp);i++ {
        pct[i] = int(toInt(temp[i],10,64))
        pct_count[i] = pct[i] * keycount / 100
    }

    //Proper formatting of the value range, read from config file
    value_r := cfg.Section_Args.Value_Range
    temp = strings.Split(value_r,",")
    value_range = make([]int,len(temp))
    for i:=0;i<len(temp);i++ {
        value_range[i] = int(toInt(temp[i],10,64))
    }
    
}

// Flag Handling
func handleFlags(){
    if *fhost != "null"{
        etcdhost=*fhost
        remote_host=etcdhost
    }
    if *fport != "null"{
        etcdport=*fport
    }
    if *foperation != "null" {
        operation=*foperation
    }
    if *fkeycount != -1 {
        keycount = *fkeycount
    }
    if *foperation_count != -1 {
        operation_count = *foperation_count
    }
    if *flog_file != "null" {
        log_file = *flog_file
    }
    remote_flag = *fremote_flag
    mem_flag = *fmem_flag

    // If remote flag not set, then confirm that it is a local etcd instance.
    if mem_flag && !remote_flag {
        ipAd,_ := exec.Command("ip","addr").Output()
        ipAddr := string(ipAd)
        if ! strings.Contains(ipAddr,"inet "+etcdhost) {
            fmt.Println("Please use the remote flag for remote etcd instance")
            fmt.Println("****************************")
            mem_flag = false
        }
    }
}

// Dials to setup tcp connection to remote host, used for memory information
func dialClient(){
    t_key, _ := getKeyFile()
    if  err !=nil {
        panic(err)
    }
    key = t_key

    config := &ssh.ClientConfig{
        User: remote_host_user,
        Auth: []ssh.AuthMethod{
        ssh.PublicKeys(key),
        },
    }

    t_client,err := ssh.Dial("tcp", remote_host+":"+ssh_port, config)
    if err != nil {
        fmt.Println("\n","Failed to dial: " + err.Error())
        fmt.Println("Unable to establish connection to remote machine.")
        fmt.Println("Make sure that password-less connection is possible.")
        fmt.Println("************************************")
        mem_flag = false
        return
    }
    ssh_client = t_client
}

// Fetch memory information for remote etcd instance
func memRemote() int {
    var bits bytes.Buffer
    mem_cmd := "pmap -x $(pidof etcd) | tail -n1 | awk '{print $4}'"
    session, err := ssh_client.NewSession()
    if err != nil {
        panic("Failed to create session: " + err.Error())
    }
    defer session.Close()
    session.Stdout = &bits
    if err := session.Run(mem_cmd); err != nil {
        panic("Failed to run: " + err.Error())
    }

    pidetcd_s := bits.String()
    pidetcd_s = strings.TrimSpace(pidetcd_s)
    etcdmem_i, _ := strconv.Atoi(pidetcd_s)
    return etcdmem_i
}

// Fetch memory information for local etcd instance
func memLocal() int {
    pidtemp, _ := exec.Command("pidof","etcd").Output()
    pidetcd := string(pidtemp)
    pidetcd = strings.TrimSpace(pidetcd)
    pidetcd_s := getMemUse(pidetcd)
    etcdmem_i, _  := strconv.Atoi(pidetcd_s)
    return etcdmem_i
}

// Handle fetching and printing of memory information
func memHandler(mprint bool){
    // Memory information after handling requests
    if mprint {
        if remote_flag {
            etcdmem_e = memRemote()
        } else 
        {
            etcdmem_e = memLocal()
        }
        fmt.Println("\n\nMemory information : \n"+
            "  Memory usage by etcd before requests:",etcdmem_s," KB\n"+
            "  Memory usage by etcd after requests:",etcdmem_e, " KB\n"+
            "  Difference := ", etcdmem_e - etcdmem_s," KB")
        log.Println("Memory usage : before,after load test, difference "+
                ":=", etcdmem_s, etcdmem_e, etcdmem_e - etcdmem_s)
        return
    }

    // This part is executed only when memory information is requested, when 
    // etcd is running on a remote machine.
    if remote_flag {
        dialClient()
        if mem_flag{
            etcdmem_s = memRemote()
        }
    } else 
    {
        etcdmem_s = memLocal()
    }
}

// As the name suggests, returns a single int64 value.
func toInt(s string, base int, bitSize int) int64 {
    i, err := strconv.ParseInt(s, base, bitSize)
    if err != nil {
        panic(err)
    }
    return i
}

// Converts integer to string and return the string.
func toString(i int) string {
    s := strconv.Itoa(i)
    return s
}

// This function handles the get requests.
func get_values(base int, per_thread int){
    var key int
    limit := base + (keycount / threads)
    for i:=0;i<per_thread;i++{
        m,_ := rand.Int(rand.Reader,big.NewInt(int64(limit-base)))
        key = int(m.Int64()) + base
        start = time.Now()
        _, err := client.Get(toString(key),false,false)
        elapsed := time.Since(start)
        log.Println("key %s took %s", key, elapsed)

        // This part is helpful in generating the commandline report
        var errStr string
        if err != nil {
            errStr = err.Error()
        }
        results <- &result{
            errStr:   errStr,
            duration: elapsed,
        }
    } 
    defer wg.Done()
}

// This function handles the create requests.
func create_keys(base int, count int, r [2]int){
    var key int
    for i:=0;i<count;i++{
        key = base + i
        m,_ := rand.Int(rand.Reader,big.NewInt(int64(r[1]-r[0])))
        r1 := int(m.Int64()) + r[0]
        value := RandStringBytesRmndr(r1)
        start = time.Now()
        _, err := client.Set(toString(key),value,1000)
        elapsed := time.Since(start)
        log.Println("key %s took %s", key, elapsed)
        
        // This part is helpful in generating the commandline report
        var errStr string
        if err != nil {
            errStr = err.Error()
        }
        results <- &result{
            errStr:   errStr,
            duration: elapsed,
        }
        
    }
    defer wg.Done()
}

// This function handles the update requests.
func update_values(base int, per_thread int){
    var key int
    val := "UpdatedValue"
    limit := base + (keycount / threads)
    for i:=0;i<per_thread;i++{
        m,_ := rand.Int(rand.Reader,big.NewInt(int64(limit-base)))
        key = int(m.Int64()) + base
        start = time.Now()
        _, err := client.Set(toString(key),val,1000) 
        elapsed := time.Since(start)
        log.Println("key %s took %s", key, elapsed)

        // This part is helpful in generating the commandline report
        var errStr string
        if err != nil {
            errStr = err.Error()
        }
        results <- &result{
            errStr:   errStr,
            duration: elapsed,
        }
    }
    defer wg.Done()
}

// This function handles the delete requests.
func delete_values(base int, per_thread int){
    var key int
    limit := base + (keycount / threads)
    for i:=0;i<per_thread;i++{
        m,_ := rand.Int(rand.Reader,big.NewInt(int64(limit-base)))
        key = int(m.Int64()) + base
        start = time.Now()
        _, err := client.Delete(toString(key),false)
        elapsed := time.Since(start)
        log.Println("key %s took %s", key, elapsed)

        // This part is helpful in generating the commandline report
        var errStr string
        if err != nil {
            errStr = err.Error()
        }
        results <- &result{
            errStr:   errStr,
            duration: elapsed,
        }
    }
    defer wg.Done()
}

// This function handles calls to functions for get/update/delete except create.
func handler(fn actions){
    per_thread := operation_count/threads
    base := 0
    for i:=0;i<threads;i++{
        go fn(base,per_thread)
        base = base + (keycount/threads)
    }
}

// This function us ised to create a random string -- used as value for a key.
func RandStringBytesRmndr(n int) string {
    const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
    b := make([]byte, n)
    for i := range b {
        m,_ := rand.Int(rand.Reader,big.NewInt(int64(len(letterBytes))))
        b[i] = letterBytes[int(m.Int64())]
    }
    return string(b)
}

// This function returns memory information when etcd is running locally.
func getMemUse(pid string) string{
    mem, _ := exec.Command("pmap","-x",pid).Output()
    mempmap := string(mem)
    temp := strings.Split(mempmap,"\n")
    temp2 := temp[len(temp)-2]
    temp3 := strings.Fields(temp2)
    memory := temp3[3]
    return memory
}

// This function prints the time elapesed for a particular request.
func timeTrack(start time.Time, name string) {
    elapsed := time.Since(start)
    log.Println("key %s took %s", name, elapsed)
}

// This function is used to get the pulic ssh key to make password-less 
// connection to a remote machine.
func getKeyFile() (key ssh.Signer, err error){
    file := os.Getenv("HOME") + "/.ssh/id_rsa"
    buf, err := ioutil.ReadFile(file)
    if err != nil {
        return
    }
    key, err = ssh.ParsePrivateKey(buf)
    if err != nil {
        return
     }
    return
}
