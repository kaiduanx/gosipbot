package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	kalbi "github.com/KalbiProject/kalbi"
	"github.com/KalbiProject/kalbi/authentication"
	"github.com/KalbiProject/kalbi/sip/message"
	"github.com/KalbiProject/kalbi/sip/method"
)

type SipBot struct {
	serverUrl  string
	serverPort string
	username   string
	password   string
	localIp    string
	localPort  string
	sipStack   *kalbi.SipStack
}

func (sb *SipBot) HandleResponses(event message.SipEventObject) {
	fmt.Printf("Incoming SIP Response\n%s", event.GetSipMessage().Src)
	response := event.GetSipMessage()
	fmt.Printf("Status code %s, description %s\n", string(response.Req.StatusCode), string(response.Req.StatusDesc))
	tx := event.GetTransaction()
	switch tx.GetLastMessage().GetStatusCode() {
	case 401:
		sb.HandleUnAuth(event)
	default:
	}
}

func (sb *SipBot) HandleRequests(event message.SipEventObject) {
	fmt.Printf("Incoming SIP Request")
}

func (sb *SipBot) HandleUnAuth(event message.SipEventObject) {
	response := event.GetSipMessage()

	origin := event.GetTransaction().GetOrigin()

	//copy original auth header
	authHeader := response.Auth

	authHeader.SetCNonce("nwqlcqw80wnf")
	authHeader.SetUsername(sb.username)
	authHeader.SetNc("00000001")
	authHeader.SetURI("sip:" + sb.serverUrl)
	authHeader.SetResponse(authentication.MD5Challenge(authHeader.GetUsername(), authHeader.GetRealm(), sb.password, authHeader.GetURI(), authHeader.GetNonce(), authHeader.GetCNonce(), authHeader.GetNc(), authHeader.GetQoP(), string(origin.Req.Method)))
	origin.SetAuthHeader(&authHeader)
	if string(event.GetTransaction().GetOrigin().Req.Method) != "INVITE" {
		origin.CallID.SetValue(message.GenerateNewCallID())
	}

	txmng := sb.sipStack.GetTransactionManager()
	tx := txmng.NewClientTransaction(origin)
	tx.Send(origin, sb.serverUrl, sb.serverPort)
}

func (sb *SipBot) onInvite(event message.SipEventObject) {
	fmt.Printf("Incoming INVITE\n%s", event.GetSipMessage().Src)
}

func (sb *SipBot) onAck(event message.SipEventObject) {
	fmt.Printf("Incoming ACK")
}

func (sb *SipBot) onCancel(event message.SipEventObject) {
	fmt.Printf("Incoming CANCEL")
}

func (sb *SipBot) onBye(event message.SipEventObject) {
	fmt.Printf("Incoming BYE")
}

func (sb *SipBot) generateRegister() *message.SipMsg {
	requestLine := message.NewRequestLine(method.REGISTER, "sip", sb.username, sb.serverUrl, sb.serverPort)
	requestVia := message.NewViaHeader("udp", sb.localIp, sb.localPort)
	requestVia.SetBranch(message.GenerateBranchID())
	requestFrom := message.NewFromHeader(sb.username, "sip", sb.serverUrl, sb.serverPort)
	requestFrom.SetTag("3234jhf23")
	requestTo := message.NewToHeader(sb.username, "sip", sb.serverUrl, sb.serverPort)
	requestContact := message.NewContactHeader("sip", sb.username, sb.localIp)
	requestContact.Port = []byte(sb.localPort)
	requestCallID := message.NewCallID(message.GenerateNewCallID())
	requestCseq := message.NewCSeq("1", method.REGISTER)
	requestMaxFor := message.NewMaxForwards("70")
	requestContentLen := message.NewContentLength("0")
	request := message.NewRequest(requestLine, requestVia, requestTo, requestFrom, requestContact, requestCallID, requestCseq, requestMaxFor, requestContentLen)
	return request
}

func main() {
	sipBot := &SipBot{
		serverUrl:  "192.168.2.59",
		serverPort: "7206",
		username:   "1001",
		password:   "100672",
		localIp:    "192.168.2.59",
		localPort:  "8000",
	}
	sipStack := kalbi.NewSipStack("Goodstartsoft")
	sipBot.sipStack = sipStack
	port, _ := strconv.Atoi(sipBot.localPort)
	fmt.Printf("Local port %d\n", port)
	sipStack.CreateListenPoint("udp", sipBot.localIp, port)
	sipStack.SetSipListener(sipBot)
	sipStack.INVITE(sipBot.onInvite)
	sipStack.CANCEL(sipBot.onCancel)
	sipStack.ACK(sipBot.onAck)
	sipStack.BYE(sipBot.onBye)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		sig := <-sigChan
		fmt.Println("exit requested, shutting down, sig", sig)
		sipStack.Stop()
	}()

	go sipStack.Start()
	time.Sleep(2 * time.Second)
	register := sipBot.generateRegister()
	txmng := sipStack.GetTransactionManager()
	txmng.NewClientTransaction(register)
	fmt.Printf("Send REGISTER:\n%s", register)
	sipStack.ListeningPoints[0].Send(sipBot.serverUrl, sipBot.serverPort, register.String())
	alive := true

	for alive {
		var command string
		fmt.Print("sipbot>")

		fmt.Scanln(&command)

		switch command {
		case "exit":
			alive = false
			fmt.Println("Exiting...")
		case "":
			continue
		default:
			fmt.Println("Unknown command")
		}
	}
}
