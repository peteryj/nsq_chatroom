package main

import (
    "fmt"
    "flag"
    "bufio"
    "os"
    "os/signal"
    "syscall"
    "log"
    "time"
    "math/rand"
    "errors"
    "github.com/bitly/go-nsq"
    "github.com/bitly/nsq/util"
)

type RoomStatus int32

const (
    rmInactive RoomStatus = 0
    rmActive   RoomStatus = iota
)

var (
    nsqlookupdList = util.StringArray{}
    nsqdAddr = flag.String("nsqd-addr", "", "nsqd address.")
    userName = ""
)

func init () {
    flag.Var(&nsqlookupdList, "nsqlookupd-addr", "nsqlookupd address.")
}

type RoomHandler struct {
    status RoomStatus
    topic string
    user string
    reader *nsq.Reader
    writer *nsq.Writer
    msgChan chan string
}

type CMD struct {
    cmd byte
    buf string
}

func newRoomHandler () *RoomHandler {
    return &RoomHandler{
        status : rmInactive,
        msgChan : make(chan string),
    }
}

func getRequeueDelay(m *nsq.Message) int {
	return int(60 * time.Second * time.Duration(m.Attempts) / time.Millisecond)
}

func (rcvr *RoomHandler) HandleMessage (message *nsq.Message) error {
    talkLine := string(message.Body)
    if rcvr.msgChan == nil {
        return errors.New("no msgChan.\n")
    }

    rcvr.msgChan <- talkLine
    return nil
}

func cmdHelp () {
    fmt.Printf("=============================================\n")
    fmt.Printf("# cmd:                                      #\n")
    fmt.Printf("#    r <user> - user name                   #\n")
    fmt.Printf("#    e <room> - enter a room                #\n")
    fmt.Printf("#    l - leave                              #\n")
    fmt.Printf("#    s <something>- say something           #\n")
    fmt.Printf("#    h - help                               #\n")
    fmt.Printf("#    q - quit                               #\n")
    fmt.Printf("=============================================\n")
}

func cmdPrompt () string {
    if userName == "" {
        return fmt.Sprintf(">",)
    } else {
        return fmt.Sprintf("(%s)>", userName)
    }
}

func (rcvr *RoomHandler) roomPrompt () string{
    return fmt.Sprintf(" >> ")
}

func (rcvr *RoomHandler) cmdEnterRoom (topic string) error {
    if rcvr.status != rmInactive {
        return errors.New("not inactive.\n")
    }
    //subscribe to the topic
    hostName,_ := os.Hostname();
    rand.Seed(time.Now().UnixNano())
    channel := fmt.Sprintf("%s_%s%06d", topic, hostName, rand.Int()%999999)

    log.Printf("hostname: %s", hostName)

    reader,err := nsq.NewReader(topic, channel)
    if err != nil {
        return err
    }

    reader.AddHandler(rcvr)
    err = reader.ConnectToNSQ(*nsqdAddr)
    if err != nil {
        return err
    }

    rcvr.reader = reader
    rcvr.status = rmActive
    rcvr.topic = topic

    return nil
}

func (rcvr *RoomHandler) cmdLeaveRoom () error {

    if rcvr.reader != nil {
        rcvr.reader.Stop()
        rcvr.reader = nil
    }

    if rcvr.writer != nil {
        rcvr.writer.Stop()
        rcvr.writer = nil
    }

    rcvr.status = rmInactive

    return nil
}

func (rcvr *RoomHandler) cmdSay (cmd *CMD) error {

    if rcvr.status != rmActive {
        return errors.New("not active.\n")
    }

    //Lasily create writer
    if rcvr.writer == nil {
        rcvr.writer = nsq.NewWriter(*nsqdAddr)
    }

    talk_line := fmt.Sprintf("(%s)say:%s", rcvr.user, cmd.buf)

    _, _, err := rcvr.writer.Publish(rcvr.topic, []byte(talk_line))
    return err
}

func (rcvr *RoomHandler) cmdRegister (cmd *CMD) error {
    rcvr.user = cmd.buf
    userName = rcvr.user
    return nil
}

func (rcvr *RoomHandler) cmdHandle(cmd *CMD, quitChan chan int) {
    switch cmd.cmd {
        case 'e':
            if cmd.buf == "" {
                fmt.Printf("Expect a room name.\n")
                return
            }
            fmt.Println()
            err := rcvr.cmdEnterRoom(cmd.buf)
            if err != nil {
                fmt.Println(err.Error())
                return
            }
        case 'l':
            err := rcvr.cmdLeaveRoom()
            if err != nil {
                fmt.Printf(err.Error())
                return
            }
        case 's':
            if cmd.buf == "" {
                fmt.Printf("Expect a sentence.\n")
                return
            }
            fmt.Println()
            err := rcvr.cmdSay(cmd)
            if err != nil {
                fmt.Printf(err.Error())
                return
            }
        case 'r':
            if cmd.buf == "" {
                fmt.Printf("Expect a user id.\n")
                return
            }
            fmt.Println()
            rcvr.cmdRegister(cmd)
        case 'q':
            err := rcvr.cmdLeaveRoom()
            if err != nil {
                fmt.Printf(err.Error())
            }
            close(quitChan)
        case 'h':
            cmdHelp()
        default:
            fmt.Printf("Invalid option!\n")
    }
}

func msgPump (cmdChan chan CMD, termChan chan os.Signal, exitChan chan int) {
    quitChan := make(chan int)
    roomHandler := newRoomHandler()

    for {
        select {
        case c := <-cmdChan:
            roomHandler.cmdHandle(&c, quitChan)
        case msg := <- roomHandler.msgChan:
            fmt.Printf("%s %s\n", roomHandler.roomPrompt(), msg);
        case <-termChan:
            close(exitChan)
            return
        case <-quitChan:
            close(exitChan)
            return
        }
    }
}

func main () {
    flag.Parse()

    /*
    if len(nsqlookupdList) == 0 {
        log.Fatalf("need at least one nsqlookupd address.");
        return;
    }
    */
    if *nsqdAddr == "" { log.Fatalf("need at least one nsqd address.");
        return;
    }

    // register signal action
    termChan := make(chan os.Signal, 1)
    signal.Notify(termChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

    cmdChan := make(chan CMD)
    exitChan := make(chan int)
    go msgPump(cmdChan, termChan, exitChan)

    // enter cmd
    for {
        select{
        case <-exitChan:
            return
        default:
        }

        cmd := CMD{}

        fmt.Printf("%s", cmdPrompt())
        stdin := bufio.NewReader(os.Stdin)
        strBytes, _, err := stdin.ReadLine()
        if err != nil {
            log.Fatalf(err.Error())
        }

        if len(strBytes) == 0 {
            continue
        }

        cmd.cmd = strBytes[0]
        if len(strBytes) > 3 {
            cmd.buf = string(strBytes[2:])
        }

        cmdChan <- cmd

        time.Sleep(1)
    }
}

